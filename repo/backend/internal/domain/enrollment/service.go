// Package enrollment manages program enrollments with full eligibility evaluation
// and atomic, concurrency-safe enrollment commits.
//
// Eligibility checks (pre-transaction, fast path):
//  1. Program must be published.
//  2. Enrollment window must be open (if configured).
//  3. Reader status must allow enrollment.
//  4. Blacklist rules — reader attributes must not match any blacklist rule.
//  5. Whitelist rules — if any whitelist rules exist, reader must match at least one.
//  6. Prerequisites — reader must have a completed enrollment in each required program.
//
// Concurrency-safe checks (inside the DB transaction, under SELECT FOR UPDATE):
//  7. Capacity — confirmed seat count < program capacity.
//  8. Duplicate enrollment.
//  9. Schedule conflict — no overlapping confirmed/pending enrollment.
//
// Checks 7–9 are re-evaluated inside the transaction even if they passed in the
// pre-flight to prevent TOCTOU races.
package enrollment

import (
	"context"
	"fmt"
	"time"

	auditpkg "lms/internal/audit"
	"lms/internal/model"
)

// ReaderLoader is the minimal interface the enrollment service needs to load
// reader data for eligibility checking. Implemented by readers.Repository.
type ReaderLoader interface {
	GetByID(ctx context.Context, id, branchID string) (*model.Reader, error)
}

// ProgramLoader is the minimal interface needed to load program data and
// confirmed enrollment counts before starting the transaction.
type ProgramLoader interface {
	GetByID(ctx context.Context, id, branchID string) (*model.Program, error)
	GetPrerequisites(ctx context.Context, programID string) ([]*model.ProgramPrerequisite, error)
	GetEnrollmentRules(ctx context.Context, programID string) ([]*model.EnrollmentRule, error)
}

// Service orchestrates eligibility evaluation and atomic enrollment.
type Service struct {
	repo          Repository
	programs      ProgramLoader
	readers       ReaderLoader
	auditLogger   *auditpkg.Logger
}

// NewService creates a new enrollment Service.
func NewService(
	repo Repository,
	programs ProgramLoader,
	readers ReaderLoader,
	auditLogger *auditpkg.Logger,
) *Service {
	return &Service{
		repo:        repo,
		programs:    programs,
		readers:     readers,
		auditLogger: auditLogger,
	}
}

// ── Enroll ────────────────────────────────────────────────────────────────────

// EnrollResult includes the new enrollment and immediate feedback fields.
type EnrollResult struct {
	Enrollment *model.Enrollment
}

// EnrollReaderRequest carries parameters for a new enrollment initiated by staff or self-service.
type EnrollReaderRequest struct {
	ProgramID         string
	ReaderID          string
	BranchID          string
	EnrollmentChannel string
	ActorUserID       string
	WorkstationID     string
}

// EligibilityDenial carries the specific reason an enrollment was denied.
// It maps to an HTTP 422 response with a structured body.
type EligibilityDenial struct {
	Reason string // closed_window | not_published | reader_ineligible | blacklisted | not_whitelisted | prerequisite_not_met
	Detail string
}

func (e *EligibilityDenial) Error() string {
	return fmt.Sprintf("enrollment denied (%s): %s", e.Reason, e.Detail)
}

// Enroll evaluates eligibility and, if eligible, atomically enrolls the reader.
func (s *Service) Enroll(ctx context.Context, req EnrollReaderRequest) (*model.Enrollment, error) {
	// ── Load program ──────────────────────────────────────────────────────────
	prog, err := s.programs.GetByID(ctx, req.ProgramID, req.BranchID)
	if err != nil {
		return nil, err
	}

	// ── Check 1: program must be published ────────────────────────────────────
	if prog.Status != "published" {
		return nil, &EligibilityDenial{
			Reason: "not_published",
			Detail: fmt.Sprintf("program status is %q; enrollment requires status published", prog.Status),
		}
	}

	// ── Check 2: enrollment window ────────────────────────────────────────────
	now := time.Now().UTC()
	if prog.EnrollmentOpensAt != nil && now.Before(*prog.EnrollmentOpensAt) {
		return nil, &EligibilityDenial{
			Reason: "closed_window",
			Detail: fmt.Sprintf("enrollment opens at %s", prog.EnrollmentOpensAt.Format(time.RFC3339)),
		}
	}
	if prog.EnrollmentClosesAt != nil && now.After(*prog.EnrollmentClosesAt) {
		return nil, &EligibilityDenial{
			Reason: "closed_window",
			Detail: fmt.Sprintf("enrollment closed at %s", prog.EnrollmentClosesAt.Format(time.RFC3339)),
		}
	}

	// ── Load reader ───────────────────────────────────────────────────────────
	reader, err := s.readers.GetByID(ctx, req.ReaderID, req.BranchID)
	if err != nil {
		return nil, err
	}

	// ── Check 3: reader status allows enrollment ──────────────────────────────
	// We look up the status from the in-memory read; the DB-enforced CHECK
	// constraint on reader_statuses is the authoritative source.
	if !readerStatusAllowsEnrollment(reader.StatusCode) {
		return nil, &EligibilityDenial{
			Reason: "reader_ineligible",
			Detail: fmt.Sprintf("reader status %q does not allow enrollment", reader.StatusCode),
		}
	}

	// ── Load and evaluate enrollment rules ────────────────────────────────────
	rules, err := s.programs.GetEnrollmentRules(ctx, req.ProgramID)
	if err != nil {
		return nil, err
	}

	// Check 4: blacklist (takes precedence over whitelist).
	for _, rule := range rules {
		if rule.RuleType == "blacklist" && ruleMatchesReader(rule, reader) {
			detail := fmt.Sprintf("reader matches blacklist rule (%s=%s)", rule.MatchField, rule.MatchValue)
			if rule.Reason != nil {
				detail = *rule.Reason
			}
			return nil, &EligibilityDenial{Reason: "blacklisted", Detail: detail}
		}
	}

	// Check 5: whitelist — if any whitelist rules exist, reader must match at least one.
	var hasWhitelist bool
	var whitelistMatched bool
	for _, rule := range rules {
		if rule.RuleType == "whitelist" {
			hasWhitelist = true
			if ruleMatchesReader(rule, reader) {
				whitelistMatched = true
				break
			}
		}
	}
	if hasWhitelist && !whitelistMatched {
		return nil, &EligibilityDenial{
			Reason: "not_whitelisted",
			Detail: "reader does not meet any whitelist criteria for this program",
		}
	}

	// Check 6: prerequisites.
	prereqs, err := s.programs.GetPrerequisites(ctx, req.ProgramID)
	if err != nil {
		return nil, err
	}
	for _, prereq := range prereqs {
		completed, checkErr := s.hasCompletedProgram(ctx, req.ReaderID, prereq.RequiredProgramID)
		if checkErr != nil {
			return nil, checkErr
		}
		if !completed {
			detail := fmt.Sprintf("reader has not completed required program %s", prereq.RequiredProgramID)
			if prereq.Description != nil {
				detail = fmt.Sprintf("prerequisite not met: %s", *prereq.Description)
			}
			return nil, &EligibilityDenial{Reason: "prerequisite_not_met", Detail: detail}
		}
	}

	// ── Atomic enroll (checks 7–9 inside the transaction) ─────────────────────
	enrollment, err := s.repo.Enroll(ctx, EnrollRequest{
		ProgramID:         req.ProgramID,
		ReaderID:          req.ReaderID,
		BranchID:          req.BranchID,
		EnrollmentChannel: req.EnrollmentChannel,
		EnrolledByUserID:  req.ActorUserID,
		WorkstationID:     req.WorkstationID,
	})
	if err != nil {
		return nil, err
	}

	if s.auditLogger != nil {
		s.auditLogger.LogEnrollmentChanged(ctx,
			req.ActorUserID, "", enrollment.ID, req.ProgramID,
			"none", "confirmed", "initial enrollment",
		)
	}

	return enrollment, nil
}

// ── Drop ──────────────────────────────────────────────────────────────────────

// DropRequest carries parameters for cancelling an enrollment.
type DropRequest struct {
	EnrollmentID  string
	ReaderID      string
	BranchID      string
	Reason        string
	ActorUserID   string
}

// Drop cancels an enrollment and records the status change.
func (s *Service) Drop(ctx context.Context, req DropRequest) error {
	if err := s.repo.Drop(ctx, req.EnrollmentID, req.ReaderID, req.BranchID, req.Reason, req.ActorUserID); err != nil {
		return err
	}
	if s.auditLogger != nil {
		s.auditLogger.LogEnrollmentChanged(ctx,
			req.ActorUserID, "", req.EnrollmentID, "",
			"confirmed", "cancelled", req.Reason,
		)
	}
	return nil
}

// ── Read operations ───────────────────────────────────────────────────────────

func (s *Service) GetByID(ctx context.Context, id, branchID string) (*model.Enrollment, error) {
	return s.repo.GetByID(ctx, id, branchID)
}

func (s *Service) ListByProgram(ctx context.Context, programID, branchID string, p model.Pagination) (model.PageResult[*model.Enrollment], error) {
	return s.repo.ListByProgram(ctx, programID, branchID, p)
}

func (s *Service) ListByReader(ctx context.Context, readerID, branchID string, p model.Pagination) (model.PageResult[*model.Enrollment], error) {
	return s.repo.ListByReader(ctx, readerID, branchID, p)
}

func (s *Service) GetHistory(ctx context.Context, enrollmentID, branchID string) ([]*model.EnrollmentHistory, error) {
	// Verify the enrollment belongs to the caller's branch before returning history.
	if _, err := s.repo.GetByID(ctx, enrollmentID, branchID); err != nil {
		return nil, err
	}
	return s.repo.GetHistory(ctx, enrollmentID)
}

// ── Eligibility helpers ───────────────────────────────────────────────────────

// readerStatusAllowsEnrollment maps reader status codes to enrollment eligibility.
// This is a local mirror of the reader_statuses table; the authoritative check
// is the DB constraint but this avoids an extra round-trip.
func readerStatusAllowsEnrollment(statusCode string) bool {
	switch statusCode {
	case "active":
		return true
	default:
		// frozen, blacklisted, pending_verification → all disallow enrollment.
		return false
	}
}

// ruleMatchesReader checks whether a reader matches a single enrollment rule.
// Supported match_field values: status_code, branch_id, reader_number.
func ruleMatchesReader(rule *model.EnrollmentRule, reader *model.Reader) bool {
	switch rule.MatchField {
	case "status_code":
		return reader.StatusCode == rule.MatchValue
	case "branch_id":
		return reader.BranchID == rule.MatchValue
	case "reader_number":
		return reader.ReaderNumber == rule.MatchValue
	default:
		return false
	}
}

// hasCompletedProgram returns true if the reader has a completed enrollment
// in the given program.
func (s *Service) hasCompletedProgram(ctx context.Context, readerID, requiredProgramID string) (bool, error) {
	// Load all reader enrollments and check for a completed one in the required program.
	// Using a large page is acceptable because readers typically have few enrollments.
	page, err := s.repo.ListByReader(ctx, readerID, "", model.Pagination{Page: 1, PerPage: 200})
	if err != nil {
		return false, fmt.Errorf("check prerequisite: %w", err)
	}
	for _, e := range page.Items {
		if e.ProgramID == requiredProgramID && e.Status == "completed" {
			return true, nil
		}
	}
	return false, nil
}

// ── Capacity check (pre-flight, informational) ────────────────────────────────

// GetRemainingSeats returns the number of available seats for a program.
// This is informational only; the authoritative capacity check happens inside
// the enrollment transaction.
func (s *Service) GetRemainingSeats(ctx context.Context, programID, branchID string) (int, error) {
	prog, err := s.programs.GetByID(ctx, programID, branchID)
	if err != nil {
		return 0, err
	}
	count, err := s.repo.ConfirmedCount(ctx, programID)
	if err != nil {
		return 0, err
	}
	remaining := prog.Capacity - count
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// ── apperr mapping for EligibilityDenial ─────────────────────────────────────

// EligibilityDenialCode maps EligibilityDenial.Reason to a machine-readable error code.
func EligibilityDenialCode(reason string) int {
	// All eligibility denials return 422 — same as apperr.Validation.
	_ = reason
	return 422
}
