// Package holdings manages title-level bibliographic records.
package holdings

import (
	"context"

	"lms/internal/model"
)

// Repository defines the data access contract for holdings.
type Repository interface {
	Create(ctx context.Context, h *model.Holding) error
	GetByID(ctx context.Context, id, branchID string) (*model.Holding, error)
	Update(ctx context.Context, h *model.Holding) error
	Deactivate(ctx context.Context, id, branchID string) error
	List(ctx context.Context, branchID string, f HoldingFilter, p model.Pagination) (model.PageResult[*model.Holding], error)
}

// HoldingFilter carries optional filter parameters for the List query.
type HoldingFilter struct {
	ISBN     *string
	Category *string
	Search   *string // title or author prefix
	Active   *bool
}
