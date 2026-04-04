package model

import "time"

// StocktakeSession is a named inventory scan run for one branch.
// Only one session per branch may be open/in_progress at a time
// (enforced by the unique partial index on stocktake_sessions).
type StocktakeSession struct {
	ID        string     `json:"id"`
	BranchID  string     `json:"branch_id"`
	Name      string     `json:"name"`
	Status    string     `json:"status"` // open, in_progress, closed, cancelled
	StartedAt time.Time  `json:"started_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	StartedBy string     `json:"started_by"`
	ClosedBy  *string    `json:"closed_by,omitempty"`
	Notes     *string    `json:"notes,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// StocktakeFinding records the result of scanning one barcode during a session.
type StocktakeFinding struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"session_id"`
	CopyID         *string   `json:"copy_id,omitempty"` // nil if barcode not in system
	ScannedBarcode string    `json:"scanned_barcode"`
	FindingType    string    `json:"finding_type"` // found, missing, unexpected, damaged
	ScannedAt      time.Time `json:"scanned_at"`
	ScannedBy      *string   `json:"scanned_by,omitempty"`
	Notes          *string   `json:"notes,omitempty"`
}
