package model

import "time"

// FeedbackTag is a controlled-vocabulary label applied to feedback.
type FeedbackTag struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// Feedback is a star rating and optional tagged comment from a reader.
type Feedback struct {
	ID          string     `json:"id"`
	BranchID    string     `json:"branch_id"`
	ReaderID    string     `json:"reader_id"`
	TargetType  string     `json:"target_type"` // holding, program
	TargetID    string     `json:"target_id"`
	Rating      *int       `json:"rating,omitempty"` // 1–5
	Comment     *string    `json:"comment,omitempty"`
	Status      string     `json:"status"` // pending, approved, rejected, flagged
	ModeratedBy *string    `json:"moderated_by,omitempty"`
	ModeratedAt *time.Time `json:"moderated_at,omitempty"`
	SubmittedAt time.Time  `json:"submitted_at"`
	Tags        []string   `json:"tags,omitempty"` // populated by join, not stored directly
}

// Appeal is a reader-submitted challenge to a decision.
type Appeal struct {
	ID          string    `json:"id"`
	BranchID    string    `json:"branch_id"`
	ReaderID    string    `json:"reader_id"`
	AppealType  string    `json:"appeal_type"` // enrollment_denial, account_suspension, feedback_rejection, blacklist_removal, other
	TargetType  *string   `json:"target_type,omitempty"`
	TargetID    *string   `json:"target_id,omitempty"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"` // submitted, under_review, resolved, dismissed
	SubmittedAt time.Time `json:"submitted_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AppealArbitration is the formal decision record for an appeal.
type AppealArbitration struct {
	ID             string    `json:"id"`
	AppealID       string    `json:"appeal_id"`
	ArbitratorID   string    `json:"arbitrator_id"`
	Decision       string    `json:"decision"` // upheld, dismissed, partial
	DecisionNotes  string    `json:"decision_notes"`
	BeforeState    any       `json:"before_state,omitempty"` // JSONB snapshot
	AfterState     any       `json:"after_state,omitempty"`  // JSONB snapshot
	DecidedAt      time.Time `json:"decided_at"`
}
