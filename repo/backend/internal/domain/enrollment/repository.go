// Package enrollment manages program enrollments with atomic, concurrency-safe operations.
package enrollment

import (
	"context"

	"lms/internal/model"
)

// Repository defines the data access contract for enrollments.
// Enroll and Drop must use row-level locking (SELECT FOR UPDATE on programs)
// to prevent capacity over-subscription under concurrent requests.
type Repository interface {
	// Enroll atomically checks capacity, eligibility, and conflict, then inserts.
	// Returns apperr.Conflict if the reader is already enrolled or the program is full.
	// Must be called within a serializable or at minimum repeatable-read transaction.
	Enroll(ctx context.Context, req EnrollRequest) (*model.Enrollment, error)

	// Drop cancels an enrollment. Records the status change in enrollment_history.
	// Returns apperr.NotFound if the enrollment does not exist.
	Drop(ctx context.Context, enrollmentID, readerID, branchID, reason, changedByUserID string) error

	// GetByID returns the enrollment with the given ID.
	GetByID(ctx context.Context, id, branchID string) (*model.Enrollment, error)

	// ListByProgram returns all enrollments for a program, scoped to branchID.
	ListByProgram(ctx context.Context, programID, branchID string, p model.Pagination) (model.PageResult[*model.Enrollment], error)

	// ListByReader returns all enrollments for a reader.
	ListByReader(ctx context.Context, readerID, branchID string, p model.Pagination) (model.PageResult[*model.Enrollment], error)

	// GetHistory returns the status-change history for an enrollment.
	GetHistory(ctx context.Context, enrollmentID string) ([]*model.EnrollmentHistory, error)

	// ConfirmedCount returns the number of confirmed enrollments for a program.
	// Used to check capacity before enrolling.
	ConfirmedCount(ctx context.Context, programID string) (int, error)
}

// EnrollRequest carries all parameters for a new enrollment.
type EnrollRequest struct {
	ProgramID         string
	ReaderID          string
	BranchID          string
	EnrollmentChannel string
	EnrolledByUserID  string
	WorkstationID     string
}
