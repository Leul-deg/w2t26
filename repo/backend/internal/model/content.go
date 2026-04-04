package model

import "time"

// GovernedContent is a library-published item that goes through moderation
// before becoming visible to readers.
// Lifecycle: draft → pending_review → (approved|rejected) → published → archived
type GovernedContent struct {
	ID              string     `json:"id"`
	BranchID        string     `json:"branch_id"`
	Title           string     `json:"title"`
	ContentType     string     `json:"content_type"` // announcement, document, digital_resource, policy
	Body            *string    `json:"body,omitempty"`
	FilePath        *string    `json:"-"`          // internal filesystem path; not exposed in API
	FileName        *string    `json:"file_name,omitempty"`
	Status          string     `json:"status"` // draft, pending_review, approved, rejected, published, archived
	SubmittedBy     string     `json:"submitted_by"`
	SubmittedAt     time.Time  `json:"submitted_at"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	ArchivedAt      *time.Time `json:"archived_at,omitempty"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ModerationItem is a queue entry for content awaiting moderation review.
type ModerationItem struct {
	ID             string     `json:"id"`
	ContentID      string     `json:"content_id"`
	AssignedTo     *string    `json:"assigned_to,omitempty"`
	Status         string     `json:"status"`   // pending, in_review, decided
	Decision       *string    `json:"decision,omitempty"`        // approved, rejected
	DecisionReason *string    `json:"decision_reason,omitempty"`
	DecidedBy      *string    `json:"decided_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
