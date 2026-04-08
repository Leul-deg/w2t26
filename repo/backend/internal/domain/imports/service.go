// Package imports implements bulk import of readers and holdings from CSV/XLSX files.
//
// Lifecycle:
//
//	Upload  → parse/validate → stage rows (status: preview_ready or failed)
//	Preview → caller reviews rows and error summary
//	Commit  → atomic transaction: all rows succeed or entire import rolls back
//	Rollback → caller explicitly aborts a preview_ready job
//
// The "no partial import" guarantee is enforced in CommitJob: if any row-level
// insert returns an error, the database transaction is rolled back before the
// function returns. The job status transitions to "rolled_back".
package imports

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"

	"lms/internal/apperr"
	auditpkg "lms/internal/audit"
	"lms/internal/model"
)

// Service orchestrates the bulk import pipeline.
type Service struct {
	repo        Repository
	pool        *pgxpool.Pool
	auditLogger *auditpkg.Logger
}

// NewService creates a new imports Service.
func NewService(repo Repository, pool *pgxpool.Pool, auditLogger *auditpkg.Logger) *Service {
	return &Service{repo: repo, pool: pool, auditLogger: auditLogger}
}

// ── Row-level error ───────────────────────────────────────────────────────────

// RowError describes a validation failure on a specific row and field.
type RowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ── Upload & parse ────────────────────────────────────────────────────────────

// UploadRequest carries file content and metadata for a new import job.
type UploadRequest struct {
	BranchID      string
	ActorUserID   string
	WorkstationID string
	ImportType    string // readers | holdings
	FileName      string
	Data          []byte // raw uploaded file bytes (CSV or XLSX)
}

// UploadResult is returned after parsing and staging. If HasErrors is true,
// the import cannot be committed — the caller should download the error CSV.
type UploadResult struct {
	Job       *model.ImportJob
	HasErrors bool
	Errors    []RowError
}

const importCompletenessThresholdPercent = 100.0

// UploadAndParse parses the CSV, validates every row, stages the results, and
// returns the job with a preview-ready or failed status. The uploaded file is
// never written to disk; it is held in memory for validation only.
func (s *Service) UploadAndParse(ctx context.Context, req UploadRequest) (*UploadResult, error) {
	if req.BranchID == "" {
		return nil, &apperr.Validation{Field: "branch_id", Message: "branch is required"}
	}
	importType := strings.ToLower(req.ImportType)
	if importType != "readers" && importType != "holdings" {
		return nil, &apperr.Validation{Field: "import_type", Message: "must be readers or holdings"}
	}
	if len(req.Data) == 0 {
		return nil, &apperr.Validation{Field: "file", Message: "file is empty"}
	}

	// Create the job record (status: previewing while we parse).
	job := &model.ImportJob{
		BranchID:      req.BranchID,
		ImportType:    importType,
		Status:        "previewing",
		FileName:      req.FileName,
		UploadedBy:    req.ActorUserID,
		WorkstationID: strPtr(req.WorkstationID),
	}
	if err := s.repo.CreateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("create import job: %w", err)
	}

	// Parse and validate.
	importRows, rowErrors, parseErr := parseAndValidate(importType, req.FileName, req.Data)
	if parseErr != nil {
		// Structure-level failure (bad headers etc.) — mark job failed immediately.
		_ = s.repo.UpdateJobStatus(ctx, job.ID, "failed", 1, []RowError{{Row: 0, Field: "file", Message: parseErr.Error()}})
		job.Status = "failed"
		return &UploadResult{Job: job, HasErrors: true, Errors: []RowError{{Row: 0, Field: "file", Message: parseErr.Error()}}}, nil
	}

	// Stage all rows (valid and invalid alike — preview shows them all).
	if len(importRows) > 0 {
		if err := s.repo.CreateRows(ctx, job.ID, importRows); err != nil {
			return nil, fmt.Errorf("stage import rows: %w", err)
		}
	}

	rowCount := len(importRows)
	newStatus := "preview_ready"
	if len(rowErrors) > 0 {
		newStatus = "failed"
	}
	if err := s.repo.UpdateJobStatus(ctx, job.ID, newStatus, len(rowErrors), rowErrors); err != nil {
		return nil, fmt.Errorf("update job status: %w", err)
	}

	job.Status = newStatus
	job.RowCount = &rowCount
	job.ErrorCount = len(rowErrors)
	if len(rowErrors) > 0 {
		job.ErrorSummary = rowErrors
	}
	populateCompletenessFromRows(job, importRows)

	return &UploadResult{
		Job:       job,
		HasErrors: len(rowErrors) > 0,
		Errors:    rowErrors,
	}, nil
}

// ── Preview ───────────────────────────────────────────────────────────────────

// GetJobPreview returns the job and its staged rows (paginated).
func (s *Service) GetJobPreview(ctx context.Context, jobID, branchID string, p model.Pagination) (*model.ImportJob, model.PageResult[*model.ImportRow], error) {
	job, err := s.repo.GetJob(ctx, jobID, branchID)
	if err != nil {
		return nil, model.PageResult[*model.ImportRow]{}, err
	}
	rows, err := s.repo.ListRows(ctx, jobID, p)
	if err != nil {
		return nil, model.PageResult[*model.ImportRow]{}, err
	}
	allRows, err := s.loadAllRows(ctx, jobID)
	if err == nil {
		populateCompletenessFromRows(job, allRows)
	} else {
		populateCompletenessFromJob(job)
	}
	return job, rows, nil
}

// ListJobs returns a paginated list of import jobs for the branch.
func (s *Service) ListJobs(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.ImportJob], error) {
	result, err := s.repo.ListJobs(ctx, branchID, p)
	if err != nil {
		return result, err
	}
	for _, job := range result.Items {
		populateCompletenessFromJob(job)
	}
	return result, nil
}

// ── Commit ────────────────────────────────────────────────────────────────────

// CommitJob atomically inserts all staged rows into target tables.
// If any row fails validation or DB insert, the transaction is rolled back
// and the job status is set to rolled_back. No partial commits are possible.
func (s *Service) CommitJob(ctx context.Context, jobID, branchID, actorID string) (*model.ImportJob, error) {
	job, err := s.repo.GetJob(ctx, jobID, branchID)
	if err != nil {
		return nil, err
	}
	if job.Status != "preview_ready" {
		return nil, &apperr.Validation{
			Field:   "status",
			Message: fmt.Sprintf("job cannot be committed in status %q (must be preview_ready)", job.Status),
		}
	}
	if job.ErrorCount > 0 {
		return nil, &apperr.Validation{
			Field:   "error_count",
			Message: fmt.Sprintf("job has %d validation error(s) and cannot be committed; download the error report and fix the file", job.ErrorCount),
		}
	}

	// Load all staged rows for commit so large jobs are fully processed.
	allRows, err := s.loadAllRows(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("load rows for commit: %w", err)
	}

	// Begin a single database transaction covering all inserts.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	var commitErrors []RowError
	switch job.ImportType {
	case "readers":
		commitErrors = commitReaders(ctx, tx, job.BranchID, allRows)
	case "holdings":
		commitErrors = commitHoldings(ctx, tx, job.BranchID, allRows)
	default:
		_ = tx.Rollback(ctx)
		return nil, &apperr.Validation{Field: "import_type", Message: "unsupported import type"}
	}

	if len(commitErrors) > 0 {
		_ = tx.Rollback(ctx)
		_ = s.repo.UpdateJobStatus(ctx, job.ID, "rolled_back", len(commitErrors), commitErrors)
		job.Status = "rolled_back"
		job.ErrorCount = len(commitErrors)
		job.ErrorSummary = commitErrors
		populateCompletenessFromJob(job)
		if s.auditLogger != nil {
			s.auditLogger.LogImportEvent(ctx, actorID, "", model.AuditImportRolledBack, job.ID, job.BranchID,
				map[string]any{"import_type": job.ImportType, "error_count": len(commitErrors)})
		}
		return job, &apperr.Validation{
			Field:   "rows",
			Message: fmt.Sprintf("commit failed: %d row(s) had errors; import rolled back", len(commitErrors)),
		}
	}

	if err := tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		_ = s.repo.UpdateJobStatus(ctx, job.ID, "rolled_back", 1, []RowError{{Row: 0, Field: "transaction", Message: err.Error()}})
		job.Status = "rolled_back"
		populateCompletenessFromJob(job)
		return job, fmt.Errorf("commit transaction: %w", err)
	}

	_ = s.repo.UpdateJobStatus(ctx, job.ID, "committed", 0, nil)
	job.Status = "committed"
	populateCompletenessFromJob(job)
	if s.auditLogger != nil {
		s.auditLogger.LogImportEvent(ctx, actorID, "", model.AuditImportCommitted, job.ID, job.BranchID,
			map[string]any{"import_type": job.ImportType, "row_count": len(allRows)})
	}
	return job, nil
}

// ── Rollback ──────────────────────────────────────────────────────────────────

// RollbackJob explicitly cancels a preview_ready job. The staged rows are
// retained for audit purposes (per the 30-day cleanup policy defined in the schema).
func (s *Service) RollbackJob(ctx context.Context, jobID, branchID, actorID string) (*model.ImportJob, error) {
	job, err := s.repo.GetJob(ctx, jobID, branchID)
	if err != nil {
		return nil, err
	}
	if job.Status != "preview_ready" {
		return nil, &apperr.Validation{
			Field:   "status",
			Message: fmt.Sprintf("job cannot be rolled back in status %q", job.Status),
		}
	}
	if err := s.repo.UpdateJobStatus(ctx, job.ID, "rolled_back", job.ErrorCount, job.ErrorSummary); err != nil {
		return nil, err
	}
	job.Status = "rolled_back"
	populateCompletenessFromJob(job)
	if s.auditLogger != nil {
		s.auditLogger.LogImportEvent(ctx, actorID, "", model.AuditImportRolledBack, job.ID, job.BranchID,
			map[string]any{"import_type": job.ImportType, "reason": "manual_rollback"})
	}
	return job, nil
}

// ── Error file download ───────────────────────────────────────────────────────

// ErrorFile generates a downloadable CSV or XLSX file of row-level errors for a job.
// Columns: row_number, field, message.
func (s *Service) ErrorFile(ctx context.Context, jobID, branchID, format string) ([]byte, string, string, error) {
	job, err := s.repo.GetJob(ctx, jobID, branchID)
	if err != nil {
		return nil, "", "", err
	}
	if job.ErrorCount == 0 {
		return nil, "", "", &apperr.Validation{Field: "error_count", Message: "no errors to download"}
	}

	records := [][]string{{"row_number", "field", "message"}}

	if errs, ok := job.ErrorSummary.([]any); ok {
		for _, e := range errs {
			if m, ok := e.(map[string]any); ok {
				row := strconv.Itoa(int(toFloat64(m["row"])))
				field, _ := m["field"].(string)
				msg, _ := m["message"].(string)
				records = append(records, []string{row, field, msg})
			}
		}
	}

	prefix := job.ID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return encodeTabular(records, fmt.Sprintf("import_%s_errors", prefix), format)
}

// ── Template file download ────────────────────────────────────────────────────

// TemplateFile returns a CSV or XLSX template (header row only) for the given import type.
func TemplateFile(importType, format string) ([]byte, string, string, error) {
	var headers []string
	switch importType {
	case "readers":
		headers = readersCSVHeaders
	case "holdings":
		headers = holdingsCSVHeaders
	default:
		return nil, "", "", &apperr.Validation{Field: "import_type", Message: "unknown import type"}
	}
	return encodeTabular([][]string{headers}, importType+"_import_template", format)
}

// ── CSV parsing helpers ───────────────────────────────────────────────────────

// readersCSVHeaders defines the expected columns for a reader import.
// reader_number is optional (generated if blank). first_name and last_name are required.
var readersCSVHeaders = []string{
	"reader_number", "first_name", "last_name", "preferred_name",
	"contact_email", "contact_phone", "date_of_birth", "national_id",
	"notes", "status_code",
}

// holdingsCSVHeaders defines the expected columns for a holdings import.
// title is required. language defaults to "en" if blank.
var holdingsCSVHeaders = []string{
	"title", "author", "isbn", "publisher", "publication_year",
	"category", "subcategory", "language", "description",
}

func parseAndValidate(importType, fileName string, data []byte) ([]*model.ImportRow, []RowError, error) {
	records, err := readTabularRecords(fileName, data)
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("file has no content")
	}

	// Normalise header row.
	headers := make([]string, len(records[0]))
	for i, h := range records[0] {
		headers[i] = strings.ToLower(strings.TrimSpace(h))
	}

	switch importType {
	case "readers":
		return validateReadersRows(headers, records[1:])
	case "holdings":
		return validateHoldingsRows(headers, records[1:])
	default:
		return nil, nil, fmt.Errorf("unsupported import type: %s", importType)
	}
}

func readTabularRecords(fileName string, data []byte) ([][]string, error) {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".xlsx":
		return readXLSXRecords(data)
	default:
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("file must be valid UTF-8; re-save as UTF-8 CSV from Excel")
		}
		return readCSVRecords(data)
	}
}

func readCSVRecords(data []byte) ([][]string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV parse error: %w", err)
	}
	return records, nil
}

func readXLSXRecords(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("XLSX parse error: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("XLSX workbook has no worksheets")
	}
	return f.GetRows(sheets[0])
}

func encodeTabular(records [][]string, baseName, format string) ([]byte, string, string, error) {
	switch strings.ToLower(format) {
	case "", "csv":
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		for _, row := range records {
			if err := w.Write(row); err != nil {
				return nil, "", "", err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return nil, "", "", err
		}
		return buf.Bytes(), baseName + ".csv", "text/csv; charset=utf-8", nil
	case "xlsx":
		f := excelize.NewFile()
		sheet := f.GetSheetName(0)
		for i, row := range records {
			cell, err := excelize.CoordinatesToCellName(1, i+1)
			if err != nil {
				return nil, "", "", err
			}
			values := make([]any, len(row))
			for j, v := range row {
				values[j] = v
			}
			if err := f.SetSheetRow(sheet, cell, &values); err != nil {
				return nil, "", "", err
			}
		}
		buf, err := f.WriteToBuffer()
		if err != nil {
			return nil, "", "", err
		}
		return buf.Bytes(), baseName + ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
	default:
		return nil, "", "", &apperr.Validation{Field: "format", Message: "format must be csv or xlsx"}
	}
}

func populateCompletenessFromRows(job *model.ImportJob, rows []*model.ImportRow) {
	if job == nil {
		return
	}
	total := len(rows)
	job.ValidRowCount = 0
	job.InvalidRowCount = 0
	job.CompletenessThresholdPercent = importCompletenessThresholdPercent
	if total == 0 {
		job.CompletenessPercent = 0
		job.MeetsCompletenessThreshold = false
		return
	}
	for _, row := range rows {
		switch row.Status {
		case "valid", "committed":
			job.ValidRowCount++
		case "invalid", "rolled_back":
			job.InvalidRowCount++
		}
	}
	job.CompletenessPercent = float64(job.ValidRowCount) * 100 / float64(total)
	job.MeetsCompletenessThreshold = job.CompletenessPercent >= importCompletenessThresholdPercent
}

func populateCompletenessFromJob(job *model.ImportJob) {
	if job == nil {
		return
	}
	total := 0
	if job.RowCount != nil {
		total = *job.RowCount
	}
	job.CompletenessThresholdPercent = importCompletenessThresholdPercent
	if total == 0 {
		job.ValidRowCount = 0
		job.InvalidRowCount = 0
		job.CompletenessPercent = 0
		job.MeetsCompletenessThreshold = false
		return
	}
	job.InvalidRowCount = uniqueErrorRows(job.ErrorSummary)
	if total > job.InvalidRowCount {
		job.ValidRowCount = total - job.InvalidRowCount
	} else {
		job.ValidRowCount = 0
	}
	job.CompletenessPercent = float64(job.ValidRowCount) * 100 / float64(total)
	job.MeetsCompletenessThreshold = job.CompletenessPercent >= importCompletenessThresholdPercent
}

func uniqueErrorRows(summary any) int {
	rows := map[int]struct{}{}
	switch errs := summary.(type) {
	case []RowError:
		for _, err := range errs {
			rows[err.Row] = struct{}{}
		}
	case []any:
		for _, entry := range errs {
			if m, ok := entry.(map[string]any); ok {
				rows[int(toFloat64(m["row"]))] = struct{}{}
			}
		}
	}
	return len(rows)
}

func (s *Service) loadAllRows(ctx context.Context, jobID string) ([]*model.ImportRow, error) {
	const perPage = 200
	page := 1
	var allRows []*model.ImportRow
	for {
		result, err := s.repo.ListRows(ctx, jobID, model.Pagination{Page: page, PerPage: perPage})
		if err != nil {
			return nil, err
		}
		allRows = append(allRows, result.Items...)
		if page >= result.TotalPages || len(result.Items) == 0 {
			break
		}
		page++
	}
	if allRows == nil {
		allRows = []*model.ImportRow{}
	}
	return allRows, nil
}

// headerIndex returns a map of header name → column index.
func headerIndex(headers []string) map[string]int {
	m := make(map[string]int, len(headers))
	for i, h := range headers {
		m[h] = i
	}
	return m
}

// col returns the trimmed value for a header in a CSV record, or "" if absent.
func col(record []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

// ── Readers validator ─────────────────────────────────────────────────────────

func validateReadersRows(headers []string, records [][]string) ([]*model.ImportRow, []RowError, error) {
	required := []string{"first_name", "last_name"}
	idx := headerIndex(headers)
	for _, req := range required {
		if _, ok := idx[req]; !ok {
			return nil, nil, fmt.Errorf("required column %q not found in CSV header", req)
		}
	}

	seenNumbers := make(map[string]int) // reader_number → first row that used it
	var rows []*model.ImportRow
	var errs []RowError

	for i, rec := range records {
		rowNum := i + 2 // 1-indexed, +1 for header
		raw := recordToMap(headers, rec)
		rowErrs := validateReaderRecord(idx, rec, rowNum, seenNumbers)
		errs = append(errs, rowErrs...)

		status := "valid"
		var errDetail *string
		if len(rowErrs) > 0 {
			status = "invalid"
			msgs := make([]string, len(rowErrs))
			for j, e := range rowErrs {
				msgs[j] = e.Field + ": " + e.Message
			}
			s := strings.Join(msgs, "; ")
			errDetail = &s
		}

		// Track seen reader numbers for dedup check.
		if num := col(rec, idx, "reader_number"); num != "" && len(rowErrs) == 0 {
			seenNumbers[num] = rowNum
		}

		rows = append(rows, &model.ImportRow{
			JobID:        "", // set by caller
			RowNumber:    rowNum,
			RawData:      raw,
			ParsedData:   normaliseReaderRecord(idx, rec),
			Status:       status,
			ErrorDetails: errDetail,
		})
	}
	return rows, errs, nil
}

func validateReaderRecord(idx map[string]int, rec []string, rowNum int, seenNumbers map[string]int) []RowError {
	var errs []RowError
	firstName := col(rec, idx, "first_name")
	lastName := col(rec, idx, "last_name")

	if firstName == "" {
		errs = append(errs, RowError{Row: rowNum, Field: "first_name", Message: "first_name is required"})
	}
	if lastName == "" {
		errs = append(errs, RowError{Row: rowNum, Field: "last_name", Message: "last_name is required"})
	}
	if dob := col(rec, idx, "date_of_birth"); dob != "" {
		if !isValidDate(dob) {
			errs = append(errs, RowError{Row: rowNum, Field: "date_of_birth", Message: "must be YYYY-MM-DD"})
		}
	}
	if num := col(rec, idx, "reader_number"); num != "" {
		if firstRow, seen := seenNumbers[num]; seen {
			errs = append(errs, RowError{Row: rowNum, Field: "reader_number",
				Message: fmt.Sprintf("duplicate reader_number %q (first seen at row %d)", num, firstRow)})
		}
	}
	if status := col(rec, idx, "status_code"); status != "" {
		valid := map[string]bool{"active": true, "frozen": true, "blacklisted": true, "pending_verification": true}
		if !valid[status] {
			errs = append(errs, RowError{Row: rowNum, Field: "status_code",
				Message: fmt.Sprintf("unknown status_code %q", status)})
		}
	}
	return errs
}

func normaliseReaderRecord(idx map[string]int, rec []string) map[string]any {
	m := map[string]any{
		"first_name": col(rec, idx, "first_name"),
		"last_name":  col(rec, idx, "last_name"),
	}
	for _, f := range []string{"reader_number", "preferred_name", "contact_email",
		"contact_phone", "date_of_birth", "national_id", "notes", "status_code"} {
		if v := col(rec, idx, f); v != "" {
			m[f] = v
		}
	}
	if _, ok := m["status_code"]; !ok {
		m["status_code"] = "active"
	}
	return m
}

// ── Holdings validator ────────────────────────────────────────────────────────

func validateHoldingsRows(headers []string, records [][]string) ([]*model.ImportRow, []RowError, error) {
	idx := headerIndex(headers)
	if _, ok := idx["title"]; !ok {
		return nil, nil, fmt.Errorf("required column %q not found in CSV header", "title")
	}

	seenISBNs := make(map[string]int)
	var rows []*model.ImportRow
	var errs []RowError

	for i, rec := range records {
		rowNum := i + 2
		raw := recordToMap(headers, rec)
		rowErrs := validateHoldingRecord(idx, rec, rowNum, seenISBNs)
		errs = append(errs, rowErrs...)

		status := "valid"
		var errDetail *string
		if len(rowErrs) > 0 {
			status = "invalid"
			msgs := make([]string, len(rowErrs))
			for j, e := range rowErrs {
				msgs[j] = e.Field + ": " + e.Message
			}
			s := strings.Join(msgs, "; ")
			errDetail = &s
		}

		if isbn := col(rec, idx, "isbn"); isbn != "" && len(rowErrs) == 0 {
			seenISBNs[isbn] = rowNum
		}

		rows = append(rows, &model.ImportRow{
			RowNumber:    rowNum,
			RawData:      raw,
			ParsedData:   normaliseHoldingRecord(idx, rec),
			Status:       status,
			ErrorDetails: errDetail,
		})
	}
	return rows, errs, nil
}

func validateHoldingRecord(idx map[string]int, rec []string, rowNum int, seenISBNs map[string]int) []RowError {
	var errs []RowError
	if col(rec, idx, "title") == "" {
		errs = append(errs, RowError{Row: rowNum, Field: "title", Message: "title is required"})
	}
	if yearStr := col(rec, idx, "publication_year"); yearStr != "" {
		y, parseErr := strconv.Atoi(yearStr)
		if parseErr != nil || y < 1000 || y > time.Now().Year()+1 {
			errs = append(errs, RowError{Row: rowNum, Field: "publication_year",
				Message: fmt.Sprintf("must be a 4-digit year between 1000 and %d", time.Now().Year()+1)})
		}
	}
	if isbn := col(rec, idx, "isbn"); isbn != "" {
		if firstRow, seen := seenISBNs[isbn]; seen {
			errs = append(errs, RowError{Row: rowNum, Field: "isbn",
				Message: fmt.Sprintf("duplicate isbn %q (first seen at row %d)", isbn, firstRow)})
		}
	}
	return errs
}

func normaliseHoldingRecord(idx map[string]int, rec []string) map[string]any {
	m := map[string]any{"title": col(rec, idx, "title")}
	for _, f := range []string{"author", "isbn", "publisher", "category", "subcategory", "description"} {
		if v := col(rec, idx, f); v != "" {
			m[f] = v
		}
	}
	lang := col(rec, idx, "language")
	if lang == "" {
		lang = "en"
	}
	m["language"] = lang
	if yearStr := col(rec, idx, "publication_year"); yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil {
			m["publication_year"] = y
		}
	}
	return m
}

// ── Commit helpers ────────────────────────────────────────────────────────────

// commitReaders inserts all reader rows within tx.
// Returns commit-time errors (e.g. duplicate reader_number in DB).
func commitReaders(ctx context.Context, tx pgx.Tx, branchID string, rows []*model.ImportRow) []RowError {
	var errs []RowError
	for _, row := range rows {
		parsed, ok := row.ParsedData.(map[string]any)
		if !ok {
			errs = append(errs, RowError{Row: row.RowNumber, Field: "parsed_data", Message: "internal: missing parsed data"})
			continue
		}

		// Generate reader_number if not provided.
		num, _ := parsed["reader_number"].(string)
		if num == "" {
			num = generateReaderNumber()
		}
		status, _ := parsed["status_code"].(string)
		if status == "" {
			status = "active"
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO lms.readers
			    (branch_id, reader_number, status_code, first_name, last_name,
			     preferred_name, notes, registered_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
			branchID,
			num,
			status,
			parsed["first_name"],
			parsed["last_name"],
			nullStr(parsed, "preferred_name"),
			nullStr(parsed, "notes"),
		)
		if err != nil {
			msg := err.Error()
			if isUniqueSQLError(msg) {
				msg = fmt.Sprintf("reader_number %q already exists in the database", num)
			}
			errs = append(errs, RowError{Row: row.RowNumber, Field: "reader_number", Message: msg})
		}
	}
	return errs
}

// commitHoldings inserts all holding rows within tx.
func commitHoldings(ctx context.Context, tx pgx.Tx, branchID string, rows []*model.ImportRow) []RowError {
	var errs []RowError
	for _, row := range rows {
		parsed, ok := row.ParsedData.(map[string]any)
		if !ok {
			errs = append(errs, RowError{Row: row.RowNumber, Field: "parsed_data", Message: "internal: missing parsed data"})
			continue
		}
		lang, _ := parsed["language"].(string)
		if lang == "" {
			lang = "en"
		}
		var pubYear *int
		if y, ok := parsed["publication_year"].(float64); ok {
			yi := int(y)
			pubYear = &yi
		} else if y, ok := parsed["publication_year"].(int); ok {
			pubYear = &y
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO lms.holdings
			    (branch_id, title, author, isbn, publisher, publication_year,
			     category, subcategory, language, description)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			branchID,
			parsed["title"],
			nullStr(parsed, "author"),
			nullStr(parsed, "isbn"),
			nullStr(parsed, "publisher"),
			pubYear,
			nullStr(parsed, "category"),
			nullStr(parsed, "subcategory"),
			lang,
			nullStr(parsed, "description"),
		)
		if err != nil {
			errs = append(errs, RowError{Row: row.RowNumber, Field: "isbn", Message: err.Error()})
		}
	}
	return errs
}

// ── Utility ───────────────────────────────────────────────────────────────────

func recordToMap(headers []string, rec []string) map[string]any {
	m := make(map[string]any, len(headers))
	for i, h := range headers {
		v := ""
		if i < len(rec) {
			v = strings.TrimSpace(rec[i])
		}
		m[h] = v
	}
	return m
}

func isValidDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func isUniqueSQLError(msg string) bool {
	return strings.Contains(msg, "unique") || strings.Contains(msg, "23505")
}

func nullStr(m map[string]any, key string) *string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	return &s
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

// generateReaderNumber creates a short random reader identifier.
func generateReaderNumber() string {
	return fmt.Sprintf("R%d", time.Now().UnixNano()%1_000_000_000)
}

// ReadCSV is a thin wrapper exported for testing without an HTTP layer.
func ReadCSV(r io.Reader) ([][]string, error) {
	return csv.NewReader(r).ReadAll()
}
