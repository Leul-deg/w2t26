// Package programs manages scheduled library events with capacity and enrollment windows.
package programs

import (
	"context"
	"time"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/model"
)

// Service orchestrates program management.
type Service struct {
	repo        Repository
	auditLogger *auditpkg.Logger
}

// NewService creates a new programs Service.
func NewService(repo Repository, auditLogger *auditpkg.Logger) *Service {
	return &Service{repo: repo, auditLogger: auditLogger}
}

// ── Create ────────────────────────────────────────────────────────────────────

// CreateRequest carries input for creating a new program.
type CreateRequest struct {
	BranchID           string
	Title              string
	Description        *string
	Category           *string
	VenueType          *string
	VenueName          *string
	Capacity           int
	EnrollmentOpensAt  *time.Time
	EnrollmentClosesAt *time.Time
	StartsAt           time.Time
	EndsAt             time.Time
	EnrollmentChannel  string
	ActorUserID        string
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*model.Program, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}
	channel := req.EnrollmentChannel
	if channel == "" {
		channel = "any"
	}
	p := &model.Program{
		BranchID:           req.BranchID,
		Title:              req.Title,
		Description:        req.Description,
		Category:           req.Category,
		VenueType:          req.VenueType,
		VenueName:          req.VenueName,
		Capacity:           req.Capacity,
		EnrollmentOpensAt:  req.EnrollmentOpensAt,
		EnrollmentClosesAt: req.EnrollmentClosesAt,
		StartsAt:           req.StartsAt,
		EndsAt:             req.EndsAt,
		Status:             "draft",
		EnrollmentChannel:  channel,
		CreatedBy:          strPtr(req.ActorUserID),
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	if s.auditLogger != nil {
		s.auditLogger.LogAdminChange(ctx, req.ActorUserID, "", model.AuditProgramCreated, "program", p.ID, "",
			nil, map[string]any{"title": p.Title})
	}
	return p, nil
}

// ── Update ────────────────────────────────────────────────────────────────────

// UpdateRequest carries fields that may be changed on an existing program.
type UpdateRequest struct {
	Title              string
	Description        *string
	Category           *string
	VenueType          *string
	VenueName          *string
	Capacity           int
	EnrollmentOpensAt  *time.Time
	EnrollmentClosesAt *time.Time
	StartsAt           time.Time
	EndsAt             time.Time
	EnrollmentChannel  string
}

func (s *Service) Update(ctx context.Context, id, branchID, actorID string, req UpdateRequest) (*model.Program, error) {
	if req.Title == "" {
		return nil, &apperr.Validation{Field: "title", Message: "title is required"}
	}
	if req.Capacity < 1 {
		return nil, &apperr.Validation{Field: "capacity", Message: "capacity must be at least 1"}
	}
	p, err := s.repo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	p.Title = req.Title
	p.Description = req.Description
	p.Category = req.Category
	p.VenueType = req.VenueType
	p.VenueName = req.VenueName
	p.Capacity = req.Capacity
	p.EnrollmentOpensAt = req.EnrollmentOpensAt
	p.EnrollmentClosesAt = req.EnrollmentClosesAt
	p.StartsAt = req.StartsAt
	p.EndsAt = req.EndsAt
	if req.EnrollmentChannel != "" {
		p.EnrollmentChannel = req.EnrollmentChannel
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateStatus transitions a program to a new lifecycle status.
func (s *Service) UpdateStatus(ctx context.Context, id, branchID, actorID, status string) error {
	validStatuses := map[string]bool{
		"draft": true, "published": true, "cancelled": true, "completed": true,
	}
	if !validStatuses[status] {
		return &apperr.Validation{Field: "status", Message: "invalid status; must be draft, published, cancelled, or completed"}
	}
	return s.repo.UpdateStatus(ctx, id, branchID, status)
}

// Get returns a single program by ID.
func (s *Service) Get(ctx context.Context, id, branchID string) (*model.Program, error) {
	return s.repo.GetByID(ctx, id, branchID)
}

// List returns a paginated, filtered list of programs for a branch.
func (s *Service) List(ctx context.Context, branchID string, f ProgramFilter, p model.Pagination) (model.PageResult[*model.Program], error) {
	return s.repo.List(ctx, branchID, f, p)
}

// ── Prerequisites ─────────────────────────────────────────────────────────────

func (s *Service) GetPrerequisites(ctx context.Context, programID string) ([]*model.ProgramPrerequisite, error) {
	return s.repo.GetPrerequisites(ctx, programID)
}

func (s *Service) AddPrerequisite(ctx context.Context, programID, requiredProgramID string, description *string) (*model.ProgramPrerequisite, error) {
	if programID == requiredProgramID {
		return nil, &apperr.Validation{Field: "required_program_id", Message: "a program cannot be its own prerequisite"}
	}
	pr := &model.ProgramPrerequisite{
		ProgramID:         programID,
		RequiredProgramID: requiredProgramID,
		Description:       description,
	}
	if err := s.repo.AddPrerequisite(ctx, pr); err != nil {
		return nil, err
	}
	return pr, nil
}

func (s *Service) RemovePrerequisite(ctx context.Context, programID, requiredProgramID string) error {
	return s.repo.RemovePrerequisite(ctx, programID, requiredProgramID)
}

// ── Enrollment rules ──────────────────────────────────────────────────────────

func (s *Service) GetEnrollmentRules(ctx context.Context, programID string) ([]*model.EnrollmentRule, error) {
	return s.repo.GetEnrollmentRules(ctx, programID)
}

func (s *Service) AddEnrollmentRule(ctx context.Context, programID, ruleType, matchField, matchValue string, reason *string) (*model.EnrollmentRule, error) {
	if ruleType != "whitelist" && ruleType != "blacklist" {
		return nil, &apperr.Validation{Field: "rule_type", Message: "must be whitelist or blacklist"}
	}
	if matchField == "" {
		return nil, &apperr.Validation{Field: "match_field", Message: "match_field is required"}
	}
	if matchValue == "" {
		return nil, &apperr.Validation{Field: "match_value", Message: "match_value is required"}
	}
	rule := &model.EnrollmentRule{
		ProgramID:  programID,
		RuleType:   ruleType,
		MatchField: matchField,
		MatchValue: matchValue,
		Reason:     reason,
	}
	if err := s.repo.AddEnrollmentRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *Service) RemoveEnrollmentRule(ctx context.Context, programID, ruleID string) error {
	return s.repo.RemoveEnrollmentRule(ctx, programID, ruleID)
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateCreateRequest(req CreateRequest) error {
	if req.BranchID == "" {
		return &apperr.Validation{Field: "branch_id", Message: "branch is required"}
	}
	if req.Title == "" {
		return &apperr.Validation{Field: "title", Message: "title is required"}
	}
	if req.Capacity < 1 {
		return &apperr.Validation{Field: "capacity", Message: "capacity must be at least 1"}
	}
	if req.StartsAt.IsZero() {
		return &apperr.Validation{Field: "starts_at", Message: "starts_at is required"}
	}
	if req.EndsAt.IsZero() {
		return &apperr.Validation{Field: "ends_at", Message: "ends_at is required"}
	}
	if !req.EndsAt.After(req.StartsAt) {
		return &apperr.Validation{Field: "ends_at", Message: "ends_at must be after starts_at"}
	}
	if req.EnrollmentOpensAt != nil && req.EnrollmentClosesAt != nil {
		if !req.EnrollmentClosesAt.After(*req.EnrollmentOpensAt) {
			return &apperr.Validation{Field: "enrollment_closes_at", Message: "must be after enrollment_opens_at"}
		}
	}
	return nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
