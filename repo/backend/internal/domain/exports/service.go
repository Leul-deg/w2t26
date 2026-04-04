// Package exports generates and audits bulk data exports.
//
// Every export:
//  1. Creates an export_job record BEFORE generating data (audit-first).
//  2. Queries the target table with branch scope and any filters.
//  3. Writes CSV output to an in-memory buffer.
//  4. Finalises the export_job record with the row count and file name.
//
// No file is written to disk; the CSV bytes are returned to the caller for
// streaming to the HTTP response. Every export is logged regardless of size.
package exports

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/model"
)

// Service orchestrates data exports with full audit logging.
type Service struct {
	repo        Repository
	pool        *pgxpool.Pool
	auditLogger *auditpkg.Logger
}

// NewService creates a new exports Service.
func NewService(repo Repository, pool *pgxpool.Pool, auditLogger *auditpkg.Logger) *Service {
	return &Service{repo: repo, pool: pool, auditLogger: auditLogger}
}

// ExportRequest carries parameters for an export operation.
type ExportRequest struct {
	BranchID      string
	ActorUserID   string
	WorkstationID string
	ExportType    string // readers | holdings
	// Optional filters — passed through for audit logging.
	Filters map[string]string
}

// ExportResult contains the generated CSV and its audit record.
type ExportResult struct {
	Job      *model.ExportJob
	Data     []byte
	FileName string
}

// Export generates a CSV export for the requested type.
func (s *Service) Export(ctx context.Context, req ExportRequest) (*ExportResult, error) {
	if req.BranchID == "" {
		return nil, &apperr.Validation{Field: "branch_id", Message: "branch is required"}
	}

	// Normalise filter map for storage.
	var filtersAny any
	if len(req.Filters) > 0 {
		filtersAny = req.Filters
	}

	// Create the audit record before generating data.
	job := &model.ExportJob{
		BranchID:       req.BranchID,
		ExportType:     req.ExportType,
		FiltersApplied: filtersAny,
		ExportedBy:     req.ActorUserID,
		WorkstationID:  strPtr(req.WorkstationID),
	}
	if err := s.repo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create export job: %w", err)
	}

	// Generate CSV.
	var (
		data     []byte
		rowCount int
		err      error
	)
	switch req.ExportType {
	case "readers":
		data, rowCount, err = s.exportReaders(ctx, req.BranchID, req.Filters)
	case "holdings":
		data, rowCount, err = s.exportHoldings(ctx, req.BranchID, req.Filters)
	default:
		return nil, &apperr.Validation{Field: "export_type", Message: "must be readers or holdings"}
	}
	if err != nil {
		// Don't delete the audit record — record the failure.
		_ = s.repo.Finalise(ctx, job.ID, 0, "")
		return nil, fmt.Errorf("generate export: %w", err)
	}

	prefix := job.ID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	fileName := fmt.Sprintf("%s_export_%s.csv", req.ExportType, prefix)
	if err := s.repo.Finalise(ctx, job.ID, rowCount, fileName); err != nil {
		return nil, fmt.Errorf("finalise export job: %w", err)
	}
	job.RowCount = &rowCount
	job.FileName = &fileName

	s.auditLogger.LogExportCreated(ctx, req.ActorUserID, "", job.ID, req.ExportType, req.BranchID, rowCount)

	return &ExportResult{Job: job, Data: data, FileName: fileName}, nil
}

// ListJobs returns a paginated list of export jobs for the branch.
func (s *Service) ListJobs(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.ExportJob], error) {
	return s.repo.List(ctx, branchID, p)
}

// ── Reader export ─────────────────────────────────────────────────────────────

func (s *Service) exportReaders(ctx context.Context, branchID string, filters map[string]string) ([]byte, int, error) {
	q := `
		SELECT reader_number, status_code, first_name, last_name,
		       preferred_name, registered_at::text, created_at::text
		FROM lms.readers
		WHERE branch_id = $1
		ORDER BY last_name, first_name`
	rows, err := s.pool.Query(ctx, q, branchID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"reader_number", "status_code", "first_name", "last_name",
		"preferred_name", "registered_at", "created_at",
	})

	count := 0
	for rows.Next() {
		var (
			readerNumber, statusCode, firstName, lastName string
			preferredName                                 *string
			registeredAt, createdAt                       string
		)
		if err := rows.Scan(
			&readerNumber, &statusCode, &firstName, &lastName,
			&preferredName, &registeredAt, &createdAt,
		); err != nil {
			return nil, 0, err
		}
		_ = w.Write([]string{
			readerNumber, statusCode, firstName, lastName,
			derefStr(preferredName), registeredAt, createdAt,
		})
		count++
	}
	w.Flush()
	return buf.Bytes(), count, nil
}

// ── Holdings export ───────────────────────────────────────────────────────────

func (s *Service) exportHoldings(ctx context.Context, branchID string, filters map[string]string) ([]byte, int, error) {
	q := `
		SELECT title, author, isbn, publisher,
		       publication_year, category, subcategory, language,
		       is_active::text, created_at::text
		FROM lms.holdings
		WHERE branch_id = $1
		ORDER BY title`
	rows, err := s.pool.Query(ctx, q, branchID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"title", "author", "isbn", "publisher",
		"publication_year", "category", "subcategory", "language",
		"is_active", "created_at",
	})

	count := 0
	for rows.Next() {
		var (
			title, language, isActive, createdAt string
			author, isbn, publisher               *string
			pubYear                               *int
			category, subcategory                 *string
		)
		if err := rows.Scan(
			&title, &author, &isbn, &publisher,
			&pubYear, &category, &subcategory, &language,
			&isActive, &createdAt,
		); err != nil {
			return nil, 0, err
		}
		yearStr := ""
		if pubYear != nil {
			yearStr = strconv.Itoa(*pubYear)
		}
		_ = w.Write([]string{
			title, derefStr(author), derefStr(isbn), derefStr(publisher),
			yearStr, derefStr(category), derefStr(subcategory), language,
			isActive, createdAt,
		})
		count++
	}
	w.Flush()
	return buf.Bytes(), count, nil
}

// ── Utility ───────────────────────────────────────────────────────────────────

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
