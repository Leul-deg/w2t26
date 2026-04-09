package holdings

import (
	"context"

	"lms/internal/apperr"
	auditpkg "lms/internal/audit"
	"lms/internal/domain/copies"
	"lms/internal/model"
)

// validCopyStatuses is the set of status codes a copy may hold.
var validCopyStatuses = map[string]bool{
	"available":   true,
	"checked_out": true,
	"on_hold":     true,
	"lost":        true,
	"damaged":     true,
	"withdrawn":   true,
	"in_transit":  true,
	"processing":  true,
}

// directlySettableStatuses: statuses that staff can set via this endpoint.
// checked_out and in_transit are managed by the circulation domain.
var directlySettableStatuses = map[string]bool{
	"available":  true,
	"on_hold":    true,
	"lost":       true,
	"damaged":    true,
	"withdrawn":  true,
	"in_transit": true,
	"processing": true,
}

// Service orchestrates holdings and copy operations.
type Service struct {
	holdingRepo Repository
	copyRepo    copies.Repository
	auditLogger *auditpkg.Logger
}

// NewService creates a new holdings Service.
func NewService(holdingRepo Repository, copyRepo copies.Repository, auditLogger *auditpkg.Logger) *Service {
	return &Service{
		holdingRepo: holdingRepo,
		copyRepo:    copyRepo,
		auditLogger: auditLogger,
	}
}

// ── Holding operations ────────────────────────────────────────────────────────

// CreateHoldingRequest carries input for creating a title-level record.
type CreateHoldingRequest struct {
	BranchID        string
	Title           string
	Author          *string
	ISBN            *string
	Publisher       *string
	PublicationYear *int
	Category        *string
	Subcategory     *string
	Language        string
	Description     *string
	ActorUserID     string
}

// UpdateHoldingRequest carries changes to a holding's bibliographic data.
type UpdateHoldingRequest struct {
	Title           string
	Author          *string
	ISBN            *string
	Publisher       *string
	PublicationYear *int
	Category        *string
	Subcategory     *string
	Language        string
	Description     *string
}

// CreateHolding validates and inserts a new title-level record.
func (s *Service) CreateHolding(ctx context.Context, req CreateHoldingRequest) (*model.Holding, error) {
	if req.Title == "" {
		return nil, &apperr.Validation{Field: "title", Message: "title is required"}
	}
	if req.BranchID == "" {
		return nil, &apperr.Validation{Field: "branch_id", Message: "branch is required"}
	}
	lang := req.Language
	if lang == "" {
		lang = "en"
	}
	h := &model.Holding{
		BranchID:        req.BranchID,
		Title:           req.Title,
		Author:          req.Author,
		ISBN:            req.ISBN,
		Publisher:       req.Publisher,
		PublicationYear: req.PublicationYear,
		Category:        req.Category,
		Subcategory:     req.Subcategory,
		Language:        lang,
		Description:     req.Description,
		IsActive:        true,
	}
	if req.ActorUserID != "" {
		h.CreatedBy = &req.ActorUserID
	}
	if err := s.holdingRepo.Create(ctx, h); err != nil {
		return nil, err
	}
	s.auditLogger.LogAdminChange(ctx, req.ActorUserID, "", model.AuditHoldingCreated, "holding", h.ID, "", nil, map[string]any{"title": h.Title})
	return h, nil
}

// GetHolding fetches a holding by ID within the caller's branch scope.
func (s *Service) GetHolding(ctx context.Context, id, branchID string) (*model.Holding, error) {
	return s.holdingRepo.GetByID(ctx, id, branchID)
}

// UpdateHolding applies bibliographic changes to a holding.
func (s *Service) UpdateHolding(ctx context.Context, id, branchID, actorID string, req UpdateHoldingRequest) (*model.Holding, error) {
	if req.Title == "" {
		return nil, &apperr.Validation{Field: "title", Message: "title is required"}
	}
	h, err := s.holdingRepo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	before := map[string]any{"title": h.Title}
	h.Title = req.Title
	h.Author = req.Author
	h.ISBN = req.ISBN
	h.Publisher = req.Publisher
	h.PublicationYear = req.PublicationYear
	h.Category = req.Category
	h.Subcategory = req.Subcategory
	if req.Language != "" {
		h.Language = req.Language
	}
	h.Description = req.Description
	if err := s.holdingRepo.Update(ctx, h); err != nil {
		return nil, err
	}
	s.auditLogger.LogAdminChange(ctx, actorID, "", model.AuditHoldingUpdated, "holding", h.ID, "", before, map[string]any{"title": h.Title})
	return h, nil
}

// DeactivateHolding soft-deletes a holding.
func (s *Service) DeactivateHolding(ctx context.Context, id, branchID, actorID string) error {
	if err := s.holdingRepo.Deactivate(ctx, id, branchID); err != nil {
		return err
	}
	s.auditLogger.LogAdminChange(ctx, actorID, "", model.AuditHoldingDeactivated, "holding", id, "", nil, nil)
	return nil
}

// ListHoldings returns a paginated, filtered list of holdings.
func (s *Service) ListHoldings(ctx context.Context, branchID string, f HoldingFilter, p model.Pagination) (model.PageResult[*model.Holding], error) {
	return s.holdingRepo.List(ctx, branchID, f, p)
}

// ── Copy operations ───────────────────────────────────────────────────────────

// AddCopyRequest carries input for adding a physical copy to a holding.
type AddCopyRequest struct {
	Barcode       string
	StatusCode    string
	Condition     string
	ShelfLocation *string
	AcquiredAt    *string
	PricePaid     *float64
	Notes         *string
	ActorUserID   string
}

// UpdateCopyRequest carries changeable copy metadata (not status).
type UpdateCopyRequest struct {
	Condition     string
	ShelfLocation *string
	AcquiredAt    *string
	PricePaid     *float64
	Notes         *string
}

// AddCopy attaches a new physical copy to a holding. Barcode must be globally unique.
func (s *Service) AddCopy(ctx context.Context, holdingID, branchID, actorID string, req AddCopyRequest) (*model.Copy, error) {
	if req.Barcode == "" {
		return nil, &apperr.Validation{Field: "barcode", Message: "barcode is required"}
	}
	// Authorize through the parent holding first so callers cannot attach a copy to
	// a holding from another branch by guessing its UUID.
	if _, err := s.holdingRepo.GetByID(ctx, holdingID, branchID); err != nil {
		return nil, err
	}
	status := req.StatusCode
	if status == "" {
		status = "processing"
	}
	if !validCopyStatuses[status] {
		return nil, &apperr.Validation{Field: "status_code", Message: "invalid status code"}
	}
	cond := req.Condition
	if cond == "" {
		cond = "good"
	}

	c := &model.Copy{
		HoldingID:     holdingID,
		BranchID:      branchID,
		Barcode:       req.Barcode,
		StatusCode:    status,
		Condition:     cond,
		ShelfLocation: req.ShelfLocation,
		AcquiredAt:    req.AcquiredAt,
		PricePaid:     req.PricePaid,
		Notes:         req.Notes,
	}
	if err := s.copyRepo.Create(ctx, c); err != nil {
		return nil, err
	}
	s.auditLogger.LogCopyChange(ctx, actorID, "", model.AuditCopyCreated, c.ID, branchID,
		nil, map[string]any{"barcode": c.Barcode, "holding_id": holdingID})
	return c, nil
}

// GetCopyByID fetches a copy by UUID, optionally scoped to a branch.
func (s *Service) GetCopyByID(ctx context.Context, id, branchID string) (*model.Copy, error) {
	return s.copyRepo.GetByID(ctx, id, branchID)
}

// GetCopyByBarcode looks up a copy by barcode, scoped to branchID.
// Pass an empty branchID only for administrator callers.
func (s *Service) GetCopyByBarcode(ctx context.Context, barcode, branchID string) (*model.Copy, error) {
	return s.copyRepo.GetByBarcode(ctx, barcode, branchID)
}

// UpdateCopy applies metadata changes (condition, shelf_location, notes, price).
func (s *Service) UpdateCopy(ctx context.Context, id, branchID, actorID string, req UpdateCopyRequest) (*model.Copy, error) {
	c, err := s.copyRepo.GetByID(ctx, id, branchID)
	if err != nil {
		return nil, err
	}
	before := map[string]any{"condition": c.Condition, "shelf_location": c.ShelfLocation}
	c.Condition = req.Condition
	c.ShelfLocation = req.ShelfLocation
	c.AcquiredAt = req.AcquiredAt
	c.PricePaid = req.PricePaid
	c.Notes = req.Notes
	if err := s.copyRepo.Update(ctx, c); err != nil {
		return nil, err
	}
	s.auditLogger.LogCopyChange(ctx, actorID, "", model.AuditCopyUpdated, c.ID, branchID,
		before, map[string]any{"condition": c.Condition, "shelf_location": c.ShelfLocation})
	return c, nil
}

// UpdateCopyStatus transitions a copy to a new circulation status.
// checked_out transitions are reserved for the circulation domain; this endpoint
// allows all other status changes (administrative overrides: lost, withdrawn, etc.)
func (s *Service) UpdateCopyStatus(ctx context.Context, id, branchID, actorID, statusCode string) error {
	if !directlySettableStatuses[statusCode] {
		return &apperr.Validation{Field: "status_code", Message: "invalid or non-settable status code"}
	}
	c, err := s.copyRepo.GetByID(ctx, id, branchID)
	if err != nil {
		return err
	}
	oldStatus := c.StatusCode
	if err := s.copyRepo.UpdateStatus(ctx, id, statusCode); err != nil {
		return err
	}
	s.auditLogger.LogCopyChange(ctx, actorID, "", model.AuditCopyStatusChanged, id, branchID,
		map[string]any{"status_code": oldStatus}, map[string]any{"status_code": statusCode})
	return nil
}

// ListCopies returns copies for a holding.
func (s *Service) ListCopies(ctx context.Context, holdingID, branchID string, p model.Pagination) (model.PageResult[*model.Copy], error) {
	return s.copyRepo.List(ctx, holdingID, branchID, p)
}

// ListCopyStatuses returns all copy status lookup values.
func (s *Service) ListCopyStatuses(ctx context.Context) ([]*model.CopyStatus, error) {
	return s.copyRepo.ListStatuses(ctx)
}
