package stocktake

import (
	"context"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	"lms/internal/domain/copies"
	"lms/internal/model"
)

// Service provides stocktake business logic.
type Service struct {
	repo        Repository
	copyRepo    copies.Repository
	auditLogger *auditpkg.Logger
}

// NewService creates a new stocktake Service.
func NewService(repo Repository, copyRepo copies.Repository, auditLogger *auditpkg.Logger) *Service {
	return &Service{
		repo:        repo,
		copyRepo:    copyRepo,
		auditLogger: auditLogger,
	}
}

// CreateSession opens a new stocktake session for a branch.
// Returns apperr.Conflict if the branch already has an active session.
func (s *Service) CreateSession(ctx context.Context, branchID, actorID, name string, notes *string) (*model.StocktakeSession, error) {
	if name == "" {
		return nil, &apperr.Validation{Field: "name", Message: "session name is required"}
	}
	if branchID == "" {
		return nil, &apperr.Validation{Field: "branch_id", Message: "branch is required"}
	}
	active, err := s.repo.HasActiveSession(ctx, branchID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, &apperr.Conflict{Resource: "stocktake_session", Message: "branch already has an active stocktake session"}
	}

	session := &model.StocktakeSession{
		BranchID:  branchID,
		Name:      name,
		StartedBy: actorID,
		Notes:     notes,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	s.auditLogger.LogStocktakeEvent(ctx, actorID, "", model.AuditStocktakeCreated, session.ID, branchID)
	return session, nil
}

// GetSession returns a session scoped to the caller's branch.
func (s *Service) GetSession(ctx context.Context, id, branchID string) (*model.StocktakeSession, error) {
	return s.repo.GetSession(ctx, id, branchID)
}

// CloseSession transitions a session to "closed" or "cancelled".
func (s *Service) CloseSession(ctx context.Context, id, branchID, actorID, targetStatus string) error {
	if targetStatus != "closed" && targetStatus != "cancelled" {
		return &apperr.Validation{Field: "status", Message: "target status must be 'closed' or 'cancelled'"}
	}
	// Verify the session belongs to the caller's branch before closing.
	if _, err := s.repo.GetSession(ctx, id, branchID); err != nil {
		return err
	}
	if err := s.repo.UpdateSessionStatus(ctx, id, targetStatus, actorID); err != nil {
		return err
	}
	s.auditLogger.LogStocktakeEvent(ctx, actorID, "", model.AuditStocktakeClosed, id, branchID)
	return nil
}

// ListSessions returns paginated sessions for the branch.
func (s *Service) ListSessions(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.StocktakeSession], error) {
	return s.repo.ListSessions(ctx, branchID, p)
}

// RecordScan processes a single barcode scan in a stocktake session.
//
// Business rules:
//   - Session must be open or in_progress.
//   - If the barcode resolves to a known copy: finding_type defaults to 'found'.
//     Caller may override to 'damaged' if damage is observed.
//   - If the barcode is unknown: finding_type is forced to 'unexpected'.
//   - Duplicate scan within the same session returns the existing finding (idempotent).
func (s *Service) RecordScan(ctx context.Context, sessionID, branchID, actorID, barcode string, findingTypeOverride *string, notes *string) (*model.StocktakeFinding, error) {
	if barcode == "" {
		return nil, &apperr.Validation{Field: "barcode", Message: "barcode is required"}
	}

	// Verify session is active and belongs to the caller's branch.
	session, err := s.repo.GetSession(ctx, sessionID, branchID)
	if err != nil {
		return nil, err
	}
	if session.Status != "open" && session.Status != "in_progress" {
		return nil, &apperr.Validation{Field: "session", Message: "session is not open for scanning"}
	}

	// Idempotent: if already scanned, return the existing finding.
	if existing, err := s.repo.GetFinding(ctx, sessionID, barcode); err == nil {
		return existing, nil
	}

	// Determine copy_id and finding_type.
	finding := &model.StocktakeFinding{
		SessionID:      sessionID,
		ScannedBarcode: barcode,
		Notes:          notes,
	}
	if actorID != "" {
		finding.ScannedBy = &actorID
	}

	// Global barcode lookup (empty branchID) is intentional here: stocktake
	// must detect copies from other branches that are physically on the wrong
	// shelf, recording them as "unexpected" findings.
	copy, lookupErr := s.copyRepo.GetByBarcode(ctx, barcode, "")
	if lookupErr != nil {
		// Barcode not in system → unexpected item.
		finding.FindingType = "unexpected"
	} else {
		finding.CopyID = &copy.ID
		// Use the caller's override if valid; otherwise default to 'found'.
		ft := "found"
		if findingTypeOverride != nil && *findingTypeOverride == "damaged" {
			ft = "damaged"
		}
		finding.FindingType = ft
	}

	if err := s.repo.RecordFinding(ctx, finding); err != nil {
		return nil, err
	}
	s.auditLogger.LogStocktakeScan(ctx, actorID, "", sessionID, barcode, finding.FindingType)
	return finding, nil
}

// ListFindings returns paginated findings for a session.
func (s *Service) ListFindings(ctx context.Context, sessionID, branchID string, p model.Pagination) (model.PageResult[*model.StocktakeFinding], error) {
	// Verify session scope.
	if _, err := s.repo.GetSession(ctx, sessionID, branchID); err != nil {
		return model.PageResult[*model.StocktakeFinding]{}, err
	}
	return s.repo.ListFindings(ctx, sessionID, p)
}

// GetVariances returns the variance report for a session.
func (s *Service) GetVariances(ctx context.Context, sessionID, branchID string) ([]VarianceItem, error) {
	session, err := s.repo.GetSession(ctx, sessionID, branchID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetVariances(ctx, sessionID, session.BranchID)
}
