// Package feedback manages reader ratings and tagged comments on holdings and programs.
//
// Feedback submission requires an authenticated user acting on behalf of a reader.
// Submitted feedback starts in "pending" status and must be moderated before being visible.
// Available tags come from the feedback_tags controlled vocabulary.
package feedback

import (
	"context"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/model"
)

// Service orchestrates feedback submission and moderation.
type Service struct {
	repo        Repository
	auditLogger *auditpkg.Logger
}

// NewService creates a new feedback Service.
func NewService(repo Repository, auditLogger *auditpkg.Logger) *Service {
	return &Service{repo: repo, auditLogger: auditLogger}
}

// SubmitRequest carries parameters for a new feedback submission.
type SubmitRequest struct {
	BranchID    string
	ReaderID    string
	TargetType  string   // holding, program
	TargetID    string
	Rating      *int
	Comment     *string
	TagNames    []string // resolved to IDs by this service
	ActorUserID string
}

// ModerateRequest carries parameters for a moderation action on a feedback item.
type ModerateRequest struct {
	ID          string
	BranchID    string
	Status      string // approved, rejected, flagged
	ActorUserID string
}

// Submit creates a new pending feedback item.
// Tag names are resolved to IDs via ListTags.
func (s *Service) Submit(ctx context.Context, req SubmitRequest) (*model.Feedback, error) {
	if !validTargetType(req.TargetType) {
		return nil, &apperr.Validation{Field: "target_type", Message: "target_type must be holding or program"}
	}
	if req.TargetID == "" {
		return nil, &apperr.Validation{Field: "target_id", Message: "target_id is required"}
	}
	if req.ReaderID == "" {
		return nil, &apperr.Validation{Field: "reader_id", Message: "reader_id is required"}
	}
	if req.Rating != nil && (*req.Rating < 1 || *req.Rating > 5) {
		return nil, &apperr.Validation{Field: "rating", Message: "rating must be between 1 and 5"}
	}
	if req.Rating == nil && req.Comment == nil {
		return nil, &apperr.Validation{Message: "at least one of rating or comment is required"}
	}

	// Resolve tag names to IDs.
	tagIDs, err := s.resolveTagIDs(ctx, req.TagNames)
	if err != nil {
		return nil, err
	}

	f := &model.Feedback{
		BranchID:   req.BranchID,
		ReaderID:   req.ReaderID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Rating:     req.Rating,
		Comment:    req.Comment,
		Tags:       req.TagNames,
		Status:     "pending",
	}
	if err := s.repo.Create(ctx, f, tagIDs); err != nil {
		return nil, err
	}

	if s.auditLogger != nil {
		s.auditLogger.LogFeedbackSubmitted(ctx, req.ActorUserID, "", f.ID, req.ReaderID, req.TargetType, req.TargetID)
	}

	return f, nil
}

// Moderate approves, rejects, or flags a feedback item.
func (s *Service) Moderate(ctx context.Context, req ModerateRequest) (*model.Feedback, error) {
	if !validFeedbackStatus(req.Status) {
		return nil, &apperr.Validation{Field: "status", Message: "status must be approved, rejected, or flagged"}
	}

	f, err := s.repo.GetByID(ctx, req.ID, req.BranchID)
	if err != nil {
		return nil, err
	}
	if f.Status == "approved" || f.Status == "rejected" {
		return nil, &apperr.Conflict{Resource: "feedback", Message: "feedback has already been decided"}
	}

	if err := s.repo.Moderate(ctx, req.ID, req.Status, req.ActorUserID); err != nil {
		return nil, err
	}
	f.Status = req.Status
	f.ModeratedBy = strPtr(req.ActorUserID)
	if s.auditLogger != nil {
		s.auditLogger.LogFeedbackModerated(ctx, req.ActorUserID, "", req.ID, req.Status)
	}
	return f, nil
}

// Get returns a single feedback item.
func (s *Service) Get(ctx context.Context, id, branchID string) (*model.Feedback, error) {
	return s.repo.GetByID(ctx, id, branchID)
}

// List returns a paginated list of feedback items.
func (s *Service) List(ctx context.Context, branchID string, f FeedbackFilter, p model.Pagination) (model.PageResult[*model.Feedback], error) {
	return s.repo.List(ctx, branchID, f, p)
}

// ListTags returns all active feedback tags.
func (s *Service) ListTags(ctx context.Context) ([]*model.FeedbackTag, error) {
	return s.repo.ListTags(ctx)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (s *Service) resolveTagIDs(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	tags, err := s.repo.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]string, len(tags))
	for _, t := range tags {
		byName[t.Name] = t.ID
	}
	ids := make([]string, 0, len(names))
	for _, name := range names {
		id, ok := byName[name]
		if !ok {
			return nil, &apperr.Validation{Field: "tags", Message: "unknown tag: " + name}
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func validTargetType(t string) bool {
	return t == "holding" || t == "program"
}

func validFeedbackStatus(s string) bool {
	switch s {
	case "approved", "rejected", "flagged":
		return true
	}
	return false
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
