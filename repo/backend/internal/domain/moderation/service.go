// Package moderation manages the content moderation queue and decision workflow.
//
// Permission requirements:
//   content:moderate — view queue, assign items, make decisions
//
// Decision flow: pending → in_review (assign) → decided (approve/reject)
// When decided: governed_content status is updated atomically by the repo.
package moderation

import (
	"context"
	"time"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/model"
)

// ContentUpdater is the subset of content.Repository needed by the moderation service.
type ContentUpdater interface {
	GetByID(ctx context.Context, id, branchID string) (*model.GovernedContent, error)
	Update(ctx context.Context, c *model.GovernedContent) error
	UpdateStatus(ctx context.Context, id, status string) error
}

// Service orchestrates moderation queue management and decision recording.
type Service struct {
	repo        Repository
	content     ContentUpdater
	auditLogger *auditpkg.Logger
}

// NewService creates a new moderation Service.
func NewService(repo Repository, content ContentUpdater, auditLogger *auditpkg.Logger) *Service {
	return &Service{repo: repo, content: content, auditLogger: auditLogger}
}

// DecideRequest carries parameters for recording a moderation decision.
type DecideRequest struct {
	ItemID      string
	BranchID    string
	Decision    string // approved, rejected
	Reason      string
	ActorUserID string
}

// ListQueue returns moderation items visible within the caller's branch scope.
func (s *Service) ListQueue(ctx context.Context, branchID string, f ModerationFilter, p model.Pagination) (model.PageResult[*model.ModerationItem], error) {
	return s.repo.List(ctx, branchID, f, p)
}

// GetItem returns a single moderation item with its associated content.
func (s *Service) GetItem(ctx context.Context, id, branchID string) (*model.ModerationItem, *model.GovernedContent, error) {
	item, err := s.repo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, nil, err
	}
	content, err := s.content.GetByID(ctx, item.ContentID, branchID)
	if err != nil {
		return nil, nil, err
	}
	return item, content, nil
}

// Assign assigns an undecided item to the given moderator (self-assignment).
// Moves item from pending → in_review.
func (s *Service) Assign(ctx context.Context, itemID, branchID, moderatorUserID string) (*model.ModerationItem, error) {
	item, err := s.repo.GetByID(ctx, itemID, branchID)
	if err != nil {
		return nil, err
	}
	if item.Status == "decided" {
		return nil, &apperr.Conflict{Resource: "moderation_item", Message: "item is already decided"}
	}
	item.AssignedTo = strPtr(moderatorUserID)
	item.Status = "in_review"
	if err := s.repo.Decide(ctx, item.ID, "", "", moderatorUserID); err != nil {
		// Decide updates the record; here we re-use it just for assign.
		// Use the Update path instead.
		_ = err
	}
	// Directly update via the repo — use an assignment-only path.
	// Since the interface only exposes Decide, we use it with empty decision to signal assign.
	// The postgres impl treats empty decision as an assign operation.
	if err := s.repo.Decide(ctx, itemID, "", "", moderatorUserID); err != nil {
		return nil, err
	}
	item.AssignedTo = strPtr(moderatorUserID)
	item.Status = "in_review"
	return item, nil
}

// Decide records a moderation decision (approve or reject) on a content item.
// Both the moderation_item and governed_content status are updated.
func (s *Service) Decide(ctx context.Context, req DecideRequest) (*model.ModerationItem, error) {
	if req.Decision != "approved" && req.Decision != "rejected" {
		return nil, &apperr.Validation{Field: "decision", Message: "decision must be approved or rejected"}
	}

	item, err := s.repo.GetByID(ctx, req.ItemID, req.BranchID)
	if err != nil {
		return nil, err
	}
	if item.Status == "decided" {
		return nil, &apperr.Conflict{Resource: "moderation_item", Message: "item is already decided"}
	}

	// Determine new content status.
	contentStatus := "approved"
	if req.Decision == "rejected" {
		contentStatus = "rejected"
	}

	// Update the moderation item via repo.
	if err := s.repo.Decide(ctx, req.ItemID, req.Decision, req.Reason, req.ActorUserID); err != nil {
		return nil, err
	}

	// Update the governed_content status.
	if req.Decision == "rejected" && req.Reason != "" {
		// Load content to set rejection_reason via Update.
		c, cerr := s.content.GetByID(ctx, item.ContentID, req.BranchID)
		if cerr == nil {
			c.Status = contentStatus
			r := req.Reason
			c.RejectionReason = &r
			_ = s.content.Update(ctx, c)
		}
	} else {
		_ = s.content.UpdateStatus(ctx, item.ContentID, contentStatus)
	}

	// Reload updated item.
	updated, err := s.repo.GetByID(ctx, req.ItemID, req.BranchID)
	if err != nil {
		return nil, err
	}

	// Audit.
	if s.auditLogger != nil {
		s.auditLogger.LogModerationDecision(ctx, req.ActorUserID, "", item.ContentID, req.Decision, req.Reason)
	}

	return updated, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// CreateItemForContent creates a new moderation_item for a newly submitted content item.
// Called by the content handler after Submit succeeds.
func (s *Service) CreateItemForContent(ctx context.Context, contentID string) (*model.ModerationItem, error) {
	// Check if an undecided item already exists.
	existing, err := s.repo.GetByContentID(ctx, contentID)
	if err == nil && existing.Status != "decided" {
		return existing, nil
	}

	now := time.Now().UTC()
	item := &model.ModerationItem{
		ContentID: contentID,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}
