package stocktake

import (
	"context"

	"lms/internal/model"
)

// Repository defines the data access contract for stocktake sessions and findings.
type Repository interface {
	CreateSession(ctx context.Context, s *model.StocktakeSession) error
	GetSession(ctx context.Context, id, branchID string) (*model.StocktakeSession, error)
	UpdateSessionStatus(ctx context.Context, id, status, closedByUserID string) error
	ListSessions(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.StocktakeSession], error)
	RecordFinding(ctx context.Context, f *model.StocktakeFinding) error
	// GetFinding returns the finding for a barcode within a session (for idempotent scans).
	GetFinding(ctx context.Context, sessionID, barcode string) (*model.StocktakeFinding, error)
	ListFindings(ctx context.Context, sessionID string, p model.Pagination) (model.PageResult[*model.StocktakeFinding], error)
	// GetVariances returns discrepancies: missing copies, unexpected barcodes, misplaced copies.
	GetVariances(ctx context.Context, sessionID, branchID string) ([]VarianceItem, error)
	// HasActiveSession returns true when the branch already has an open or in_progress session.
	HasActiveSession(ctx context.Context, branchID string) (bool, error)
}

// VarianceItem represents a discrepancy detected during variance analysis.
type VarianceItem struct {
	// Type is one of: "missing" | "unexpected" | "misplaced"
	Type       string  `json:"type"`
	Barcode    string  `json:"barcode"`
	CopyID     *string `json:"copy_id,omitempty"`
	HoldingID  *string `json:"holding_id,omitempty"`
	Title      *string `json:"title,omitempty"`
	Author     *string `json:"author,omitempty"`
	StatusCode *string `json:"status_code,omitempty"`
	// HomeBranchID is populated for misplaced copies.
	HomeBranchID *string `json:"home_branch_id,omitempty"`
	Notes        *string `json:"notes,omitempty"`
}
