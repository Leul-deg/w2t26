// Package readers manages library patron profiles.
package readers

import (
	"context"
	"time"

	"lms/internal/model"
)

// Repository defines the data access contract for reader profiles.
// All methods enforce branch-scoped access via branchID.
// When branchID is empty, no branch filter is applied (administrator scope).
type Repository interface {
	// Create inserts a new reader. Sensitive fields must already be encrypted
	// before calling Create. Returns apperr.Conflict if reader_number exists.
	Create(ctx context.Context, r *model.Reader) error

	// GetByID returns the reader with the given ID, scoped to branchID.
	// Returns apperr.NotFound if not found or not in caller's branch.
	GetByID(ctx context.Context, id, branchID string) (*model.Reader, error)

	// GetByReaderNumber returns the reader with the given card number,
	// scoped to branchID.
	GetByReaderNumber(ctx context.Context, number, branchID string) (*model.Reader, error)

	// Update persists changes to an existing reader.
	// Sensitive fields must already be encrypted.
	Update(ctx context.Context, r *model.Reader) error

	// UpdateStatus changes the reader's status_code.
	// Returns apperr.NotFound if the reader does not exist in the caller's branch.
	UpdateStatus(ctx context.Context, id, branchID, statusCode string) error

	// List returns a paginated list of readers for the given branch.
	// Encrypted sensitive fields are included in the returned structs;
	// the service layer handles masking before constructing API responses.
	List(ctx context.Context, branchID string, filter ReaderFilter, p model.Pagination) (model.PageResult[*model.Reader], error)

	// ListStatuses returns all reader status lookup values.
	ListStatuses(ctx context.Context) ([]*model.ReaderStatus, error)

	// GetLoanHistory returns paginated circulation events for the reader,
	// joined with copy barcode and holding title/author.
	GetLoanHistory(ctx context.Context, readerID, branchID string, p model.Pagination) (model.PageResult[*LoanHistoryItem], error)

	// GetCurrentHoldings returns the reader's currently checked-out items
	// (checkout events with no subsequent return and copy still checked_out).
	GetCurrentHoldings(ctx context.Context, readerID, branchID string) ([]*LoanHistoryItem, error)
}

// ReaderFilter carries optional filter parameters for the List query.
type ReaderFilter struct {
	StatusCode *string
	Search     *string // matches first_name, last_name, reader_number (case-insensitive)
}

// LoanHistoryItem is a denormalised view of a circulation event enriched with
// copy and holding information. Used for reader loan history and current holdings.
type LoanHistoryItem struct {
	EventID   string     `json:"event_id"`
	CopyID    string     `json:"copy_id"`
	Barcode   string     `json:"barcode"`
	Title     string     `json:"title"`
	Author    *string    `json:"author,omitempty"`
	EventType string     `json:"event_type"`
	DueDate   *string    `json:"due_date,omitempty"`   // DATE as string YYYY-MM-DD
	ReturnedAt *time.Time `json:"returned_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
