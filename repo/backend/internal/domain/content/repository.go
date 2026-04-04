package content

import (
	"context"

	"lms/internal/model"
)

type Repository interface {
	Create(ctx context.Context, c *model.GovernedContent) error
	GetByID(ctx context.Context, id, branchID string) (*model.GovernedContent, error)
	Update(ctx context.Context, c *model.GovernedContent) error
	UpdateStatus(ctx context.Context, id, status string) error
	List(ctx context.Context, branchID string, f ContentFilter, p model.Pagination) (model.PageResult[*model.GovernedContent], error)
}

type ContentFilter struct {
	Status      *string
	ContentType *string
}
