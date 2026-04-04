// Package circulation handles copy checkout, return, and event history.
package circulation

import (
	"context"
	"time"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/domain/copies"
	"lms/internal/model"
)

// Service orchestrates circulation workflows.
type Service struct {
	repo       Repository
	copyRepo   copies.Repository
	auditLogger *auditpkg.Logger
}

// NewService constructs a new circulation Service.
func NewService(repo Repository, copyRepo copies.Repository, auditLogger *auditpkg.Logger) *Service {
	return &Service{repo: repo, copyRepo: copyRepo, auditLogger: auditLogger}
}

// CheckoutRequest holds parameters for a checkout operation.
type CheckoutRequest struct {
	// Provide either CopyID or Barcode — Barcode triggers a lookup.
	CopyID        string
	Barcode       string
	ReaderID      string
	BranchID      string
	DueDate       string // YYYY-MM-DD
	WorkstationID string
	Notes         string
	ActorUserID   string
}

// ReturnRequest holds parameters for a return operation.
type ReturnRequest struct {
	// Provide either CopyID or Barcode — Barcode triggers a lookup.
	CopyID        string
	Barcode       string
	BranchID      string
	WorkstationID string
	Notes         string
	ActorUserID   string
}

// Checkout records a checkout event, validates copy availability, and
// atomically transitions the copy to "checked_out".
func (s *Service) Checkout(ctx context.Context, req CheckoutRequest) (*model.CirculationEvent, error) {
	if req.ReaderID == "" {
		return nil, &apperr.Validation{Field: "reader_id", Message: "reader_id is required"}
	}
	if req.DueDate == "" {
		return nil, &apperr.Validation{Field: "due_date", Message: "due_date is required"}
	}
	if req.BranchID == "" {
		return nil, &apperr.Validation{Field: "branch_id", Message: "branch_id is required"}
	}

	copyID, err := s.resolveCopyID(ctx, req.CopyID, req.Barcode, req.BranchID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	performer := &req.ActorUserID
	if req.ActorUserID == "" {
		performer = nil
	}
	ws := optString(req.WorkstationID)
	notes := optString(req.Notes)

	e := &model.CirculationEvent{
		CopyID:        copyID,
		ReaderID:      req.ReaderID,
		BranchID:      req.BranchID,
		EventType:     "checkout",
		DueDate:       &req.DueDate,
		PerformedBy:   performer,
		WorkstationID: ws,
		Notes:         notes,
		CreatedAt:     now,
	}

	if err := s.repo.Checkout(ctx, e); err != nil {
		return nil, err
	}

	if s.auditLogger != nil {
		s.auditLogger.Log(ctx, &model.AuditEvent{
			EventType:    model.AuditCirculationCheckout,
			ActorUserID:  performer,
			BranchID:     &req.BranchID,
			ResourceType: strPtr("copy"),
			ResourceID:   &copyID,
			Metadata:     map[string]any{"reader_id": req.ReaderID, "due_date": req.DueDate},
		})
	}

	return e, nil
}

// Return records a return event and atomically transitions the copy back to "available".
func (s *Service) Return(ctx context.Context, req ReturnRequest) (*model.CirculationEvent, error) {
	if req.BranchID == "" {
		return nil, &apperr.Validation{Field: "branch_id", Message: "branch_id is required"}
	}

	copyID, err := s.resolveCopyID(ctx, req.CopyID, req.Barcode, req.BranchID)
	if err != nil {
		return nil, err
	}

	// Verify an active checkout exists before recording the return.
	active, err := s.repo.GetActiveCheckout(ctx, copyID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	performer := &req.ActorUserID
	if req.ActorUserID == "" {
		performer = nil
	}
	ws := optString(req.WorkstationID)
	notes := optString(req.Notes)

	e := &model.CirculationEvent{
		CopyID:        copyID,
		ReaderID:      active.ReaderID,
		BranchID:      req.BranchID,
		EventType:     "return",
		ReturnedAt:    &now,
		PerformedBy:   performer,
		WorkstationID: ws,
		Notes:         notes,
		CreatedAt:     now,
	}

	if err := s.repo.Return(ctx, e); err != nil {
		return nil, err
	}

	if s.auditLogger != nil {
		s.auditLogger.Log(ctx, &model.AuditEvent{
			EventType:    model.AuditCirculationReturn,
			ActorUserID:  performer,
			BranchID:     &req.BranchID,
			ResourceType: strPtr("copy"),
			ResourceID:   &copyID,
			Metadata:     map[string]any{"reader_id": active.ReaderID},
		})
	}

	return e, nil
}

// ListByBranch returns paginated circulation events for the caller's branch.
// An empty branchID returns events across all branches (administrator scope);
// the repo applies a conditional WHERE branch_id clause.
func (s *Service) ListByBranch(ctx context.Context, branchID string, f CirculationFilter, p model.Pagination) (model.PageResult[*model.CirculationEvent], error) {
	return s.repo.ListByBranch(ctx, branchID, f, p)
}

// ListByCopy returns paginated events for a specific copy.
func (s *Service) ListByCopy(ctx context.Context, copyID, branchID string, p model.Pagination) (model.PageResult[*model.CirculationEvent], error) {
	return s.repo.ListByCopy(ctx, copyID, branchID, p)
}

// ListByReader returns paginated events for a specific reader.
func (s *Service) ListByReader(ctx context.Context, readerID, branchID string, p model.Pagination) (model.PageResult[*model.CirculationEvent], error) {
	return s.repo.ListByReader(ctx, readerID, branchID, p)
}

// GetActiveCheckout returns the active checkout event for a copy, scoped to branchID.
// branchID="" (administrator) skips the branch ownership check.
// The internal repo.GetActiveCheckout call is intentionally unscoped; the caller
// (HTTP handler) passes branchID to this method for access control.
func (s *Service) GetActiveCheckout(ctx context.Context, copyID, branchID string) (*model.CirculationEvent, error) {
	// Validate the copy belongs to the caller's branch before exposing its checkout state.
	if _, err := s.copyRepo.GetByID(ctx, copyID, branchID); err != nil {
		return nil, err
	}
	return s.repo.GetActiveCheckout(ctx, copyID)
}

// resolveCopyID returns the copy UUID from either a direct ID or a branch-scoped barcode lookup.
// When copyID is provided it is validated against branchID via GetByID to prevent cross-branch
// IDOR — a caller in branch A must not be able to operate on a copy in branch B by guessing its
// UUID. GetByID accepts empty branchID for administrator callers (no branch filter applied).
func (s *Service) resolveCopyID(ctx context.Context, copyID, barcode, branchID string) (string, error) {
	if copyID != "" {
		c, err := s.copyRepo.GetByID(ctx, copyID, branchID)
		if err != nil {
			return "", err
		}
		return c.ID, nil
	}
	if barcode == "" {
		return "", &apperr.Validation{Field: "copy_id", Message: "copy_id or barcode is required"}
	}
	c, err := s.copyRepo.GetByBarcode(ctx, barcode, branchID)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func strPtr(s string) *string { return &s }
