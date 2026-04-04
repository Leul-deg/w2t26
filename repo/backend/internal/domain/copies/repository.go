// Package copies manages copy-level physical inventory.
package copies

import (
	"context"

	"lms/internal/model"
)

// Repository defines the data access contract for physical copies.
// Every copy has a globally unique barcode.
type Repository interface {
	// Create inserts a new copy. Returns apperr.Conflict if barcode already exists.
	Create(ctx context.Context, c *model.Copy) error
	GetByID(ctx context.Context, id, branchID string) (*model.Copy, error)
	// GetByBarcode looks up a copy by its barcode.
	// branchID non-empty: scopes to that branch (user-facing HTTP paths).
	// branchID empty:     global lookup (stocktake — intentionally cross-branch to detect misplaced copies).
	GetByBarcode(ctx context.Context, barcode, branchID string) (*model.Copy, error)
	Update(ctx context.Context, c *model.Copy) error
	UpdateStatus(ctx context.Context, id, statusCode string) error
	List(ctx context.Context, holdingID, branchID string, p model.Pagination) (model.PageResult[*model.Copy], error)
	ListStatuses(ctx context.Context) ([]*model.CopyStatus, error)
}
