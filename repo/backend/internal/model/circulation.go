package model

import "time"

// CirculationEvent records a single lifecycle event for a copy.
// The event_type field determines which additional fields are meaningful.
type CirculationEvent struct {
	ID                  string     `json:"id"`
	CopyID              string     `json:"copy_id"`
	ReaderID            string     `json:"reader_id"`
	BranchID            string     `json:"branch_id"`
	EventType           string     `json:"event_type"` // checkout, return, renewal, hold_placed, hold_cancelled, transit_out, transit_in
	DueDate             *string    `json:"due_date,omitempty"`
	ReturnedAt          *time.Time `json:"returned_at,omitempty"`
	DestinationBranchID *string    `json:"destination_branch_id,omitempty"`
	PerformedBy         *string    `json:"performed_by,omitempty"`
	WorkstationID       *string    `json:"workstation_id,omitempty"`
	Notes               *string    `json:"notes,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}
