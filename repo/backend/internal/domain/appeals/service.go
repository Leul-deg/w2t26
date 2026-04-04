// Package appeals manages reader appeals against moderation decisions, enrollment
// denials, account actions, and similar adverse outcomes.
//
// State machine:
//   submitted → under_review (claimed by moderator)
//   under_review → resolved | dismissed (arbitration decision)
//
// Arbitration records are immutable once written.
package appeals

import (
	"context"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/model"
)

// Service orchestrates appeal submission and arbitration.
type Service struct {
	repo        Repository
	auditLogger *auditpkg.Logger
}

// NewService creates a new appeals Service.
func NewService(repo Repository, auditLogger *auditpkg.Logger) *Service {
	return &Service{repo: repo, auditLogger: auditLogger}
}

// SubmitRequest carries parameters for a new appeal.
type SubmitRequest struct {
	BranchID    string
	ReaderID    string
	AppealType  string
	TargetType  *string
	TargetID    *string
	Reason      string
	ActorUserID string
}

// ArbitrateRequest carries the arbitration outcome.
type ArbitrateRequest struct {
	AppealID      string
	BranchID      string
	Decision      string // upheld, dismissed, partial
	DecisionNotes string
	BeforeState   any
	AfterState    any
	ActorUserID   string
}

// Submit creates a new appeal in submitted status.
func (s *Service) Submit(ctx context.Context, req SubmitRequest) (*model.Appeal, error) {
	if !validAppealType(req.AppealType) {
		return nil, &apperr.Validation{Field: "appeal_type", Message: "appeal_type must be enrollment_denial, account_suspension, feedback_rejection, blacklist_removal, or other"}
	}
	if req.Reason == "" {
		return nil, &apperr.Validation{Field: "reason", Message: "reason is required"}
	}
	if req.ReaderID == "" {
		return nil, &apperr.Validation{Field: "reader_id", Message: "reader_id is required"}
	}

	a := &model.Appeal{
		BranchID:   req.BranchID,
		ReaderID:   req.ReaderID,
		AppealType: req.AppealType,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Reason:     req.Reason,
		Status:     "submitted",
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Review transitions an appeal from submitted → under_review.
func (s *Service) Review(ctx context.Context, id, branchID, actorUserID string) (*model.Appeal, error) {
	a, err := s.repo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if a.Status != "submitted" {
		return nil, &apperr.Conflict{Resource: "appeal", Message: "only submitted appeals may be moved to under_review"}
	}
	if err := s.repo.UpdateStatus(ctx, id, "under_review"); err != nil {
		return nil, err
	}
	a.Status = "under_review"
	return a, nil
}

// Arbitrate records a final decision on an appeal.
// Transitions under_review → resolved (upheld/partial) or dismissed.
func (s *Service) Arbitrate(ctx context.Context, req ArbitrateRequest) (*model.Appeal, *model.AppealArbitration, error) {
	if !validArbitrationDecision(req.Decision) {
		return nil, nil, &apperr.Validation{Field: "decision", Message: "decision must be upheld, dismissed, or partial"}
	}
	if req.DecisionNotes == "" {
		return nil, nil, &apperr.Validation{Field: "decision_notes", Message: "decision_notes is required"}
	}

	a, err := s.repo.GetByID(ctx, req.AppealID, req.BranchID)
	if err != nil {
		return nil, nil, err
	}
	if a.Status != "under_review" {
		return nil, nil, &apperr.Conflict{Resource: "appeal", Message: "only under_review appeals may be arbitrated"}
	}

	// Determine terminal appeal status.
	newStatus := "resolved"
	if req.Decision == "dismissed" {
		newStatus = "dismissed"
	}

	arb := &model.AppealArbitration{
		AppealID:      req.AppealID,
		ArbitratorID:  req.ActorUserID,
		Decision:      req.Decision,
		DecisionNotes: req.DecisionNotes,
		BeforeState:   req.BeforeState,
		AfterState:    req.AfterState,
	}
	if err := s.repo.Arbitrate(ctx, arb); err != nil {
		return nil, nil, err
	}

	// Update appeal status.
	if err := s.repo.UpdateStatus(ctx, req.AppealID, newStatus); err != nil {
		return nil, nil, err
	}
	a.Status = newStatus

	// Audit.
	if s.auditLogger != nil {
		s.auditLogger.LogAppealDecided(ctx, req.ActorUserID, "", req.AppealID, req.Decision, req.DecisionNotes)
	}

	return a, arb, nil
}

// Get returns a single appeal.
func (s *Service) Get(ctx context.Context, id, branchID string) (*model.Appeal, *model.AppealArbitration, error) {
	a, err := s.repo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, nil, err
	}
	// Load arbitration if available.
	arb, err := s.repo.GetArbitration(ctx, id)
	if err != nil {
		// No arbitration yet — not an error.
		arb = nil
	}
	return a, arb, nil
}

// List returns a paginated list of appeals.
func (s *Service) List(ctx context.Context, branchID string, f AppealFilter, p model.Pagination) (model.PageResult[*model.Appeal], error) {
	return s.repo.List(ctx, branchID, f, p)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func validAppealType(t string) bool {
	switch t {
	case "enrollment_denial", "account_suspension", "feedback_rejection", "blacklist_removal", "other":
		return true
	}
	return false
}

func validArbitrationDecision(d string) bool {
	return d == "upheld" || d == "dismissed" || d == "partial"
}
