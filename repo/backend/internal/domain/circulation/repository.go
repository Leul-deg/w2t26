package circulation

import (
	"context"

	"lms/internal/model"
)

// Repository defines the data access contract for circulation events.
type Repository interface {
	// Checkout atomically records a checkout event and sets the copy status
	// to "checked_out". Returns apperr.Conflict if the copy is not available.
	Checkout(ctx context.Context, e *model.CirculationEvent) error

	// Return atomically records a return event and sets the copy status back
	// to "available". Returns apperr.NotFound if no active checkout exists.
	Return(ctx context.Context, e *model.CirculationEvent) error

	// Record inserts a raw circulation event without touching copy status.
	// Used for renewals, holds, and transit events.
	Record(ctx context.Context, e *model.CirculationEvent) error

	// GetActiveCheckout returns the most recent unresolved checkout for a copy.
	// Returns apperr.NotFound if the copy has no active checkout.
	GetActiveCheckout(ctx context.Context, copyID string) (*model.CirculationEvent, error)

	// ListByBranch returns paginated events for a branch with optional filters.
	ListByBranch(ctx context.Context, branchID string, f CirculationFilter, p model.Pagination) (model.PageResult[*model.CirculationEvent], error)

	// ListByCopy returns paginated events for a specific copy.
	ListByCopy(ctx context.Context, copyID, branchID string, p model.Pagination) (model.PageResult[*model.CirculationEvent], error)

	// ListByReader returns paginated events for a specific reader.
	ListByReader(ctx context.Context, readerID, branchID string, p model.Pagination) (model.PageResult[*model.CirculationEvent], error)
}

// CirculationFilter carries optional filter parameters for ListByBranch.
type CirculationFilter struct {
	EventType *string
	ReaderID  *string
	CopyID    *string
}
