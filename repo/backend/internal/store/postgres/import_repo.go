package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/imports"
	"lms/internal/model"
)

// ImportRepo implements imports.Repository against the lms schema.
type ImportRepo struct {
	pool *pgxpool.Pool
}

// NewImportRepo creates a new ImportRepo.
func NewImportRepo(pool *pgxpool.Pool) *ImportRepo {
	return &ImportRepo{pool: pool}
}

// CreateJob inserts a new import job and populates its ID and UploadedAt.
func (r *ImportRepo) CreateJob(ctx context.Context, job *model.ImportJob) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO lms.import_jobs
		    (branch_id, import_type, status, file_name, file_path,
		     row_count, error_count, error_summary, uploaded_by, workstation_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10)
		RETURNING id::text, uploaded_at`,
		job.BranchID, job.ImportType, job.Status, job.FileName, job.FilePath,
		job.RowCount, job.ErrorCount, marshalJSON(job.ErrorSummary),
		job.UploadedBy, job.WorkstationID,
	).Scan(&job.ID, &job.UploadedAt)
}

// GetJob returns the import job by ID, scoped to branchID.
func (r *ImportRepo) GetJob(ctx context.Context, id, branchID string) (*model.ImportJob, error) {
	q := `
		SELECT id::text, branch_id::text, import_type, status, file_name, file_path,
		       row_count, error_count, error_summary,
		       uploaded_by::text, uploaded_at, committed_at, rolled_back_at,
		       workstation_id
		FROM lms.import_jobs
		WHERE id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND branch_id = $2"
		args = append(args, branchID)
	}

	job := &model.ImportJob{}
	var errSummaryRaw []byte
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&job.ID, &job.BranchID, &job.ImportType, &job.Status,
		&job.FileName, &job.FilePath,
		&job.RowCount, &job.ErrorCount, &errSummaryRaw,
		&job.UploadedBy, &job.UploadedAt, &job.CommittedAt, &job.RolledBackAt,
		&job.WorkstationID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "import_job", ID: id}
		}
		return nil, err
	}
	if len(errSummaryRaw) > 0 {
		var v any
		if jsonErr := json.Unmarshal(errSummaryRaw, &v); jsonErr == nil {
			job.ErrorSummary = v
		}
	}
	return job, nil
}

// UpdateJobStatus updates the job's status, error_count, error_summary, and
// the appropriate timestamp (committed_at or rolled_back_at).
func (r *ImportRepo) UpdateJobStatus(ctx context.Context, id, status string, errorCount int, errorSummary any) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE lms.import_jobs
		SET    status        = $2::text,
		       error_count   = $3,
		       error_summary = $4::jsonb,
		       committed_at  = CASE WHEN $2::text = 'committed'    THEN NOW() ELSE committed_at  END,
		       rolled_back_at= CASE WHEN $2::text IN ('rolled_back','failed') THEN NOW() ELSE rolled_back_at END
		WHERE  id = $1`,
		id, status, errorCount, marshalJSON(errorSummary),
	)
	return err
}

// ListJobs returns a paginated list of import jobs for a branch.
func (r *ImportRepo) ListJobs(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.ImportJob], error) {
	baseWhere := ""
	args := []any{}
	if branchID != "" {
		baseWhere = "WHERE branch_id = $1"
		args = append(args, branchID)
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM lms.import_jobs "+baseWhere, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.ImportJob]{}, err
	}

	args = append(args, p.Limit(), p.Offset())
	limitPos := len(args) - 1
	offsetPos := len(args)
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, branch_id::text, import_type, status, file_name, file_path,
		       row_count, error_count, error_summary,
		       uploaded_by::text, uploaded_at, committed_at, rolled_back_at,
		       workstation_id
		FROM   lms.import_jobs
		`+baseWhere+`
		ORDER  BY uploaded_at DESC
		LIMIT  $`+itoa(limitPos)+` OFFSET $`+itoa(offsetPos), args...)
	if err != nil {
		return model.PageResult[*model.ImportJob]{}, err
	}
	defer rows.Close()

	var jobs []*model.ImportJob
	for rows.Next() {
		job := &model.ImportJob{}
		var errSummaryRaw []byte
		if err := rows.Scan(
			&job.ID, &job.BranchID, &job.ImportType, &job.Status,
			&job.FileName, &job.FilePath,
			&job.RowCount, &job.ErrorCount, &errSummaryRaw,
			&job.UploadedBy, &job.UploadedAt, &job.CommittedAt, &job.RolledBackAt,
			&job.WorkstationID,
		); err != nil {
			return model.PageResult[*model.ImportJob]{}, err
		}
		if len(errSummaryRaw) > 0 {
			var v any
			if jsonErr := json.Unmarshal(errSummaryRaw, &v); jsonErr == nil {
				job.ErrorSummary = v
			}
		}
		jobs = append(jobs, job)
	}
	if jobs == nil {
		jobs = []*model.ImportJob{}
	}
	return model.NewPageResult(jobs, total, p), nil
}

// CreateRows stages parsed import rows for a job.
func (r *ImportRepo) CreateRows(ctx context.Context, jobID string, rows []*model.ImportRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, row := range rows {
		batch.Queue(`
			INSERT INTO lms.import_rows
			    (job_id, row_number, raw_data, parsed_data, status, error_details)
			VALUES ($1, $2, $3::jsonb, $4::jsonb, $5, $6)`,
			jobID, row.RowNumber,
			marshalJSON(row.RawData), marshalJSON(row.ParsedData),
			row.Status, row.ErrorDetails,
		)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range rows {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// ListRows returns staged rows for a job with pagination.
func (r *ImportRepo) ListRows(ctx context.Context, jobID string, p model.Pagination) (model.PageResult[*model.ImportRow], error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM lms.import_rows WHERE job_id = $1", jobID,
	).Scan(&total); err != nil {
		return model.PageResult[*model.ImportRow]{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, job_id::text, row_number, raw_data, parsed_data, status, error_details, created_at
		FROM   lms.import_rows
		WHERE  job_id = $1
		ORDER  BY row_number
		LIMIT  $2 OFFSET $3`,
		jobID, p.Limit(), p.Offset(),
	)
	if err != nil {
		return model.PageResult[*model.ImportRow]{}, err
	}
	defer rows.Close()

	var result []*model.ImportRow
	for rows.Next() {
		row := &model.ImportRow{}
		var rawDataBytes, parsedDataBytes []byte
		if err := rows.Scan(
			&row.ID, &row.JobID, &row.RowNumber,
			&rawDataBytes, &parsedDataBytes,
			&row.Status, &row.ErrorDetails, &row.CreatedAt,
		); err != nil {
			return model.PageResult[*model.ImportRow]{}, err
		}
		if len(rawDataBytes) > 0 {
			var v any
			if jsonErr := json.Unmarshal(rawDataBytes, &v); jsonErr == nil {
				row.RawData = v
			}
		}
		if len(parsedDataBytes) > 0 {
			var v any
			if jsonErr := json.Unmarshal(parsedDataBytes, &v); jsonErr == nil {
				row.ParsedData = v
			}
		}
		result = append(result, row)
	}
	if result == nil {
		result = []*model.ImportRow{}
	}
	return model.NewPageResult(result, total, p), nil
}

// Ensure ImportRepo satisfies the interface at compile time.
var _ imports.Repository = (*ImportRepo)(nil)
