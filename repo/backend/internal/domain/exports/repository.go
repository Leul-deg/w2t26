package exports

import (
	"context"

	"lms/internal/model"
)

// Repository defines the data access contract for audited export records.
// A record is created before file generation; RowCount and FileName are
// updated after the file is written.
type Repository interface {
	Create(ctx context.Context, job *model.ExportJob) error
	Finalise(ctx context.Context, id string, rowCount int, fileName string) error
	List(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.ExportJob], error)
}
