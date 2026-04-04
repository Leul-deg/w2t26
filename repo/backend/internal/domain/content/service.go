// Package content manages the governed content lifecycle.
//
// Lifecycle: draft → pending_review → approved | rejected → published → archived
//
// Allowed transitions (enforced by this service):
//   draft        → pending_review  (Submit, requires content:submit)
//   pending_review → draft         (Retract, requires content:submit by author)
//   approved     → published       (Publish, requires content:publish)
//   published    → archived        (Archive, requires content:publish)
package content

import (
	"context"
	"time"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/model"
)

// Service orchestrates governed content creation and lifecycle transitions.
type Service struct {
	repo            Repository
	moderationQueue moderationQueueCreator
	auditLogger     *auditpkg.Logger
}

type moderationQueueCreator interface {
	CreateItemForContent(ctx context.Context, contentID string) (*model.ModerationItem, error)
}

// NewService creates a new content Service.
func NewService(repo Repository, moderationQueue moderationQueueCreator, auditLogger *auditpkg.Logger) *Service {
	return &Service{repo: repo, moderationQueue: moderationQueue, auditLogger: auditLogger}
}

// CreateRequest carries fields for creating a new draft content item.
type CreateRequest struct {
	BranchID    string
	Title       string
	ContentType string
	Body        *string
	FileName    *string
	ActorUserID string
}

// UpdateRequest carries fields for editing a draft item.
type UpdateRequest struct {
	ID          string
	BranchID    string
	Title       *string
	Body        *string
	FileName    *string
	ActorUserID string
}

// Create creates a new draft governed content item.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.GovernedContent, error) {
	if req.Title == "" {
		return nil, &apperr.Validation{Field: "title", Message: "title is required"}
	}
	if !validContentType(req.ContentType) {
		return nil, &apperr.Validation{Field: "content_type", Message: "content_type must be one of announcement, document, digital_resource, policy"}
	}

	c := &model.GovernedContent{
		BranchID:    req.BranchID,
		Title:       req.Title,
		ContentType: req.ContentType,
		Body:        req.Body,
		FileName:    req.FileName,
		Status:      "draft",
		SubmittedBy: req.ActorUserID,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Update edits a draft content item. Only draft items may be edited.
func (s *Service) Update(ctx context.Context, req UpdateRequest) (*model.GovernedContent, error) {
	c, err := s.repo.GetByID(ctx, req.ID, req.BranchID)
	if err != nil {
		return nil, err
	}
	if c.Status != "draft" {
		return nil, &apperr.Conflict{Resource: "content_item", Message: "only draft items may be edited"}
	}
	if req.Title != nil && *req.Title == "" {
		return nil, &apperr.Validation{Field: "title", Message: "title cannot be empty"}
	}
	if req.Title != nil {
		c.Title = *req.Title
	}
	if req.Body != nil {
		c.Body = req.Body
	}
	if req.FileName != nil {
		c.FileName = req.FileName
	}
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Submit transitions a draft item to pending_review and ensures a moderation queue entry exists.
func (s *Service) Submit(ctx context.Context, id, branchID, actorUserID string) (*model.GovernedContent, error) {
	c, err := s.repo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if c.SubmittedBy != actorUserID {
		return nil, &apperr.Forbidden{Action: "submit", Resource: "content_item"}
	}
	if c.Status != "draft" {
		return nil, &apperr.Conflict{Resource: "content_item", Message: "only draft items may be submitted for review"}
	}
	if err := s.repo.UpdateStatus(ctx, id, "pending_review"); err != nil {
		return nil, err
	}
	c.Status = "pending_review"

	if s.moderationQueue != nil {
		if _, err := s.moderationQueue.CreateItemForContent(ctx, id); err != nil {
			_ = s.repo.UpdateStatus(ctx, id, "draft")
			c.Status = "draft"
			return nil, err
		}
	}

	if s.auditLogger != nil {
		s.auditLogger.LogContentSubmitted(ctx, actorUserID, "", id)
	}
	return c, nil
}

// Retract moves a pending_review item back to draft so the author can revise it.
func (s *Service) Retract(ctx context.Context, id, branchID, actorUserID string) (*model.GovernedContent, error) {
	c, err := s.repo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if c.SubmittedBy != actorUserID {
		return nil, &apperr.Forbidden{Action: "retract", Resource: "content_item"}
	}
	if c.Status != "pending_review" {
		return nil, &apperr.Conflict{Resource: "content_item", Message: "only pending_review items may be retracted"}
	}
	if err := s.repo.UpdateStatus(ctx, id, "draft"); err != nil {
		return nil, err
	}
	c.Status = "draft"
	return c, nil
}

// Publish transitions an approved item to published.
func (s *Service) Publish(ctx context.Context, id, branchID, actorUserID string) (*model.GovernedContent, error) {
	c, err := s.repo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if c.Status != "approved" {
		return nil, &apperr.Conflict{Resource: "content_item", Message: "only approved items may be published"}
	}
	now := time.Now().UTC()
	c.PublishedAt = &now
	c.Status = "published"
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Archive transitions a published item to archived.
func (s *Service) Archive(ctx context.Context, id, branchID, actorUserID string) (*model.GovernedContent, error) {
	c, err := s.repo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	if c.Status != "published" {
		return nil, &apperr.Conflict{Resource: "content_item", Message: "only published items may be archived"}
	}
	now := time.Now().UTC()
	c.ArchivedAt = &now
	c.Status = "archived"
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Get returns a single governed content item.
func (s *Service) Get(ctx context.Context, id, branchID string) (*model.GovernedContent, error) {
	return s.repo.GetByID(ctx, id, branchID)
}

// List returns a paginated list of governed content items.
func (s *Service) List(ctx context.Context, branchID string, f ContentFilter, p model.Pagination) (model.PageResult[*model.GovernedContent], error) {
	return s.repo.List(ctx, branchID, f, p)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func validContentType(ct string) bool {
	switch ct {
	case "announcement", "document", "digital_resource", "policy":
		return true
	}
	return false
}
