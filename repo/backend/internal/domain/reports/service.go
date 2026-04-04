package reports

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/domain/exports"
	"lms/internal/model"
)

// Service orchestrates report generation, aggregate caching, and audited CSV export.
type Service struct {
	repo        Repository
	exportRepo  exports.Repository
	auditLogger *auditpkg.Logger
}

// NewService constructs a new reports Service.
func NewService(repo Repository, exportRepo exports.Repository, auditLogger *auditpkg.Logger) *Service {
	return &Service{repo: repo, exportRepo: exportRepo, auditLogger: auditLogger}
}

// RunRequest holds parameters for a live report query.
type RunRequest struct {
	BranchID     string
	DefinitionID string
	From         time.Time
	To           time.Time
	Filters      map[string]string
	ActorUserID  string
}

// RunResult is the output of a live query.
type RunResult struct {
	Definition *model.ReportDefinition `json:"definition"`
	From       time.Time               `json:"from"`
	To         time.Time               `json:"to"`
	Rows       []map[string]any        `json:"rows"`
	RowCount   int                     `json:"row_count"`
}

// ExportReportRequest holds parameters for a CSV report export.
type ExportReportRequest struct {
	BranchID     string
	DefinitionID string
	From         time.Time
	To           time.Time
	Filters      map[string]string
	ActorUserID  string
}

// ExportReportResult wraps the generated CSV bytes and the audit record.
type ExportReportResult struct {
	Job      *model.ExportJob
	Data     []byte
	FileName string
}

// RecalcRequest holds parameters for an on-demand aggregate recalculation.
type RecalcRequest struct {
	BranchID     string
	DefinitionID string // empty = all definitions
	From         time.Time
	To           time.Time
	ActorUserID  string
}

// ListDefinitions returns all active report definitions.
// Branch is not a filter here; definitions are branch-agnostic templates.
func (s *Service) ListDefinitions(ctx context.Context) ([]*model.ReportDefinition, error) {
	return s.repo.ListDefinitions(ctx)
}

// GetDefinition retrieves a single definition by ID.
func (s *Service) GetDefinition(ctx context.Context, id string) (*model.ReportDefinition, error) {
	if id == "" {
		return nil, &apperr.Validation{Field: "id", Message: "definition id is required"}
	}
	return s.repo.GetDefinition(ctx, id)
}

// RunReport executes a live query for the given definition and returns rows.
// Results are branch-scoped to req.BranchID and filtered to [req.From, req.To].
func (s *Service) RunReport(ctx context.Context, req RunRequest) (*RunResult, error) {
	if req.BranchID == "" {
		return nil, &apperr.Validation{Field: "branch_id", Message: "branch is required"}
	}
	if req.From.IsZero() || req.To.IsZero() {
		return nil, &apperr.Validation{Field: "period", Message: "from and to dates are required"}
	}
	if req.To.Before(req.From) {
		return nil, &apperr.Validation{Field: "period", Message: "to must not be before from"}
	}

	def, err := s.repo.GetDefinition(ctx, req.DefinitionID)
	if err != nil {
		return nil, err
	}
	if !def.IsActive {
		return nil, &apperr.Validation{Field: "definition_id", Message: "report definition is not active"}
	}

	rows, err := s.repo.RunLiveQuery(ctx, req.BranchID, def.QueryTemplate, req.Filters, req.From, req.To)
	if err != nil {
		return nil, err
	}

	return &RunResult{
		Definition: def,
		From:       req.From,
		To:         req.To,
		Rows:       rows,
		RowCount:   len(rows),
	}, nil
}

// ListAggregates returns cached aggregates for a definition over a date range.
func (s *Service) ListAggregates(ctx context.Context, branchID, definitionID string, from, to time.Time) ([]*model.ReportAggregate, error) {
	return s.repo.ListAggregates(ctx, branchID, definitionID, from, to)
}

// RecalculateAggregates recomputes and caches aggregates for the given request.
// If DefinitionID is empty, all active definitions are recalculated.
// Each calendar day in [From, To] produces one aggregate record.
//
// BranchID may be empty: an empty string is a sentinel meaning "all branches"
// and is used exclusively by the nightly scheduler to produce global aggregates.
// User-facing callers (handler, direct API) must always supply a non-empty BranchID;
// the handler enforces this via the branch scope middleware before reaching this method.
func (s *Service) RecalculateAggregates(ctx context.Context, req RecalcRequest) (int, error) {
	if req.From.IsZero() || req.To.IsZero() {
		return 0, &apperr.Validation{Field: "period", Message: "from and to are required"}
	}
	if req.To.Before(req.From) {
		return 0, &apperr.Validation{Field: "period", Message: "to must not be before from"}
	}

	var defs []*model.ReportDefinition
	if req.DefinitionID != "" {
		d, err := s.repo.GetDefinition(ctx, req.DefinitionID)
		if err != nil {
			return 0, err
		}
		defs = []*model.ReportDefinition{d}
	} else {
		var err error
		defs, err = s.repo.ListDefinitions(ctx)
		if err != nil {
			return 0, err
		}
	}

	computed := 0
	// Iterate over each day in the range.
	for d := req.From.Truncate(24 * time.Hour); !d.After(req.To); d = d.AddDate(0, 0, 1) {
		dayEnd := d.Add(24*time.Hour - time.Second)
		periodStart := d.Format("2006-01-02")
		periodEnd := dayEnd.Format("2006-01-02")

		for _, def := range defs {
			if !def.IsActive {
				continue
			}
			rows, err := s.repo.RunLiveQuery(ctx, req.BranchID, def.QueryTemplate, nil, d, dayEnd)
			if err != nil {
				// Log but continue; partial failure should not abort the batch.
				continue
			}

			data, _ := json.Marshal(rows)
			agg := &model.ReportAggregate{
				ReportDefinitionID: def.ID,
				BranchID:           &req.BranchID,
				PeriodStart:        periodStart,
				PeriodEnd:          periodEnd,
				AggregateData:      json.RawMessage(data),
			}
			if err := s.repo.UpsertAggregate(ctx, agg); err == nil {
				computed++
			}
		}
	}

	return computed, nil
}

// ExportReport runs a live query and streams the results as CSV.
// An export_job audit record is created before and finalised after generation.
func (s *Service) ExportReport(ctx context.Context, req ExportReportRequest) (*ExportReportResult, error) {
	if req.BranchID == "" {
		return nil, &apperr.Validation{Field: "branch_id", Message: "branch is required"}
	}
	if req.ActorUserID == "" {
		return nil, &apperr.Validation{Field: "actor", Message: "actor user id is required"}
	}
	if req.From.IsZero() || req.To.IsZero() {
		return nil, &apperr.Validation{Field: "period", Message: "from and to are required"}
	}

	def, err := s.repo.GetDefinition(ctx, req.DefinitionID)
	if err != nil {
		return nil, err
	}

	// Serialize filters for audit record — created BEFORE file generation.
	filterJSON, _ := json.Marshal(req.Filters)
	job := &model.ExportJob{
		BranchID:      req.BranchID,
		ExportType:    "report",
		FiltersApplied: json.RawMessage(filterJSON),
		ExportedBy:    req.ActorUserID,
	}
	if err := s.exportRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create export job: %w", err)
	}

	// Run the live query.
	rows, err := s.repo.RunLiveQuery(ctx, req.BranchID, def.QueryTemplate, req.Filters, req.From, req.To)
	if err != nil {
		// Audit record already exists — log the failure count as 0.
		_ = s.exportRepo.Finalise(ctx, job.ID, 0, "")
		return nil, err
	}

	// Build CSV.
	data, fileName, err := buildCSV(def.Name, req.From, req.To, rows)
	if err != nil {
		_ = s.exportRepo.Finalise(ctx, job.ID, 0, "")
		return nil, fmt.Errorf("build csv: %w", err)
	}

	rowCount := len(rows)
	if err := s.exportRepo.Finalise(ctx, job.ID, rowCount, fileName); err != nil {
		return nil, fmt.Errorf("finalise export job: %w", err)
	}
	job.RowCount = &rowCount
	job.FileName = &fileName
	if s.auditLogger != nil {
		s.auditLogger.LogExportCreated(ctx, req.ActorUserID, "", job.ID, "report", req.BranchID, rowCount)
	}

	return &ExportReportResult{
		Job:      job,
		Data:     data,
		FileName: fileName,
	}, nil
}

// buildCSV serialises rows to CSV bytes.
// Column order is sorted alphabetically for deterministic output.
func buildCSV(reportName string, from, to time.Time, rows []map[string]any) ([]byte, string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if len(rows) == 0 {
		fileName := fmt.Sprintf("report_%s_%s.csv", from.Format("20060102"), to.Format("20060102"))
		return buf.Bytes(), fileName, nil
	}

	// Collect headers from first row.
	headers := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		headers = append(headers, k)
	}
	sort.Strings(headers)

	if err := w.Write(headers); err != nil {
		return nil, "", err
	}
	for _, row := range rows {
		record := make([]string, len(headers))
		for i, h := range headers {
			record[i] = fmt.Sprintf("%v", row[h])
		}
		if err := w.Write(record); err != nil {
			return nil, "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}

	safeName := ""
	for _, c := range reportName {
		if c == ' ' || c == '/' || c == '\\' {
			safeName += "_"
		} else {
			safeName += string(c)
		}
	}
	fileName := fmt.Sprintf("%s_%s_%s.csv", safeName, from.Format("20060102"), to.Format("20060102"))
	return buf.Bytes(), fileName, nil
}
