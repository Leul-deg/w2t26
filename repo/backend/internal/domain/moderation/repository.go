package moderation

import (
	"context"

	"lms/internal/model"
)

type Repository interface {
	Create(ctx context.Context, item *model.ModerationItem) error
	GetByID(ctx context.Context, id, branchID string) (*model.ModerationItem, error)
	GetByContentID(ctx context.Context, contentID string) (*model.ModerationItem, error)
	Decide(ctx context.Context, id, decision, reason, decidedByUserID string) error
	List(ctx context.Context, branchID string, f ModerationFilter, p model.Pagination) (model.PageResult[*model.ModerationItem], error)
}

type ModerationFilter struct {
	Status     *string
	AssignedTo *string
}
