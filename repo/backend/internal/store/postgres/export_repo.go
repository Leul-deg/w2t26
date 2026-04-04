package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/exports"
	"lms/internal/model"
)

// ExportRepo implements exports.Repository against the lms schema.
type ExportRepo struct {
	pool *pgxpool.Pool
}

// NewExportRepo creates a new ExportRepo.
func NewExportRepo(pool *pgxpool.Pool) *ExportRepo {
	return &ExportRepo{pool: pool}
}

// Create inserts a new export job and populates its ID and ExportedAt.
func (r *ExportRepo) Create(ctx context.Context, job *model.ExportJob) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO lms.export_jobs
		    (branch_id, export_type, filters_applied, exported_by, workstation_id)
		VALUES ($1, $2, $3::jsonb, $4, $5)
		RETURNING id::text, exported_at`,
		job.BranchID, job.ExportType, marshalJSON(job.FiltersApplied),
		job.ExportedBy, job.WorkstationID,
	).Scan(&job.ID, &job.ExportedAt)
}

// Finalise updates row_count and file_name after the file has been generated.
func (r *ExportRepo) Finalise(ctx context.Context, id string, rowCount int, fileName string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE lms.export_jobs
		SET row_count = $2, file_name = $3
		WHERE id = $1`,
		id, rowCount, fileName,
	)
	return err
}

// List returns a paginated, reverse-chronological list of export jobs for a branch.
func (r *ExportRepo) List(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.ExportJob], error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM lms.export_jobs WHERE branch_id = $1", branchID,
	).Scan(&total); err != nil {
		return model.PageResult[*model.ExportJob]{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, branch_id::text, export_type, filters_applied,
		       row_count, file_name, exported_by::text, exported_at, workstation_id
		FROM   lms.export_jobs
		WHERE  branch_id = $1
		ORDER  BY exported_at DESC
		LIMIT  $2 OFFSET $3`,
		branchID, p.Limit(), p.Offset(),
	)
	if err != nil {
		return model.PageResult[*model.ExportJob]{}, err
	}
	defer rows.Close()

	var jobs []*model.ExportJob
	for rows.Next() {
		job := &model.ExportJob{}
		var filtersRaw []byte
		if err := rows.Scan(
			&job.ID, &job.BranchID, &job.ExportType, &filtersRaw,
			&job.RowCount, &job.FileName,
			&job.ExportedBy, &job.ExportedAt, &job.WorkstationID,
		); err != nil {
			return model.PageResult[*model.ExportJob]{}, err
		}
		if len(filtersRaw) > 0 {
			var v any
			if jsonErr := json.Unmarshal(filtersRaw, &v); jsonErr == nil {
				job.FiltersApplied = v
			}
		}
		jobs = append(jobs, job)
	}
	if jobs == nil {
		jobs = []*model.ExportJob{}
	}
	return model.NewPageResult(jobs, total, p), nil
}

// GetByID returns an export job by ID, scoped to the branch.
// Used in tests and potential future download-replay features.
func (r *ExportRepo) GetByID(ctx context.Context, id, branchID string) (*model.ExportJob, error) {
	q := `
		SELECT id::text, branch_id::text, export_type, filters_applied,
		       row_count, file_name, exported_by::text, exported_at, workstation_id
		FROM   lms.export_jobs
		WHERE  id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND branch_id = $2"
		args = append(args, branchID)
	}

	job := &model.ExportJob{}
	var filtersRaw []byte
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&job.ID, &job.BranchID, &job.ExportType, &filtersRaw,
		&job.RowCount, &job.FileName,
		&job.ExportedBy, &job.ExportedAt, &job.WorkstationID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "export_job", ID: id}
		}
		return nil, err
	}
	if len(filtersRaw) > 0 {
		var v any
		if jsonErr := json.Unmarshal(filtersRaw, &v); jsonErr == nil {
			job.FiltersApplied = v
		}
	}
	return job, nil
}

// Ensure ExportRepo satisfies the interface at compile time.
var _ exports.Repository = (*ExportRepo)(nil)
