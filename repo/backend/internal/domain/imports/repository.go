package imports

import (
	"context"

	"lms/internal/model"
)

// Repository defines the data access contract for bulk import jobs.
// The commit operation must use a single database transaction that rolls back
// entirely if any row fails — this is enforced in the service layer, not here.
type Repository interface {
	CreateJob(ctx context.Context, job *model.ImportJob) error
	GetJob(ctx context.Context, id, branchID string) (*model.ImportJob, error)
	UpdateJobStatus(ctx context.Context, id, status string, errorCount int, errorSummary any) error
	ListJobs(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.ImportJob], error)

	// CreateRows stages parsed rows for preview. Called after parse, before commit.
	CreateRows(ctx context.Context, jobID string, rows []*model.ImportRow) error
	// ListRows returns the staged rows for a job (for preview display).
	ListRows(ctx context.Context, jobID string, p model.Pagination) (model.PageResult[*model.ImportRow], error)
}
