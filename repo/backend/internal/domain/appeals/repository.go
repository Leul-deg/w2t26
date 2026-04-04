package appeals

import (
	"context"

	"lms/internal/model"
)

type Repository interface {
	Create(ctx context.Context, a *model.Appeal) error
	GetByID(ctx context.Context, id, branchID string) (*model.Appeal, error)
	UpdateStatus(ctx context.Context, id, status string) error
	List(ctx context.Context, branchID string, f AppealFilter, p model.Pagination) (model.PageResult[*model.Appeal], error)
	Arbitrate(ctx context.Context, arb *model.AppealArbitration) error
	GetArbitration(ctx context.Context, appealID string) (*model.AppealArbitration, error)
}

type AppealFilter struct {
	Status     *string
	AppealType *string
	ReaderID   *string
}
