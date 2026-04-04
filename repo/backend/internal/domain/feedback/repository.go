package feedback

import (
	"context"

	"lms/internal/model"
)

type Repository interface {
	Create(ctx context.Context, f *model.Feedback, tagIDs []string) error
	GetByID(ctx context.Context, id, branchID string) (*model.Feedback, error)
	Moderate(ctx context.Context, id, status, moderatedByUserID string) error
	List(ctx context.Context, branchID string, f FeedbackFilter, p model.Pagination) (model.PageResult[*model.Feedback], error)
	ListTags(ctx context.Context) ([]*model.FeedbackTag, error)
}

type FeedbackFilter struct {
	TargetType *string
	TargetID   *string
	Status     *string
}
