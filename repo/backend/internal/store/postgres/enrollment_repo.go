package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/enrollment"
	"lms/internal/model"
)

// EnrollmentRepo implements enrollment.Repository.
// Enroll uses SELECT FOR UPDATE on the programs row to prevent over-subscription
// under concurrent requests.
type EnrollmentRepo struct {
	pool *pgxpool.Pool
}

// NewEnrollmentRepo creates a new EnrollmentRepo.
func NewEnrollmentRepo(pool *pgxpool.Pool) *EnrollmentRepo {
	return &EnrollmentRepo{pool: pool}
}

const enrollmentSelectCols = `
	id::text, program_id::text, reader_id::text, branch_id::text,
	status, enrollment_channel, waitlist_position,
	enrolled_at, updated_at, enrolled_by::text`

func scanEnrollment(row pgx.Row) (*model.Enrollment, error) {
	e := &model.Enrollment{}
	err := row.Scan(
		&e.ID, &e.ProgramID, &e.ReaderID, &e.BranchID,
		&e.Status, &e.EnrollmentChannel, &e.WaitlistPosition,
		&e.EnrolledAt, &e.UpdatedAt, &e.EnrolledBy,
	)
	return e, err
}

// Enroll atomically enrolls a reader in a program.
//
// Concurrency guarantee:
//  1. BEGIN (deferred isolation applied by pgx default: READ COMMITTED).
//  2. SELECT capacity FROM programs WHERE id = $1 FOR UPDATE — acquires a row-level
//     write lock on the program row so concurrent Enroll calls serialize.
//  3. SELECT COUNT(*) confirmed enrollments — re-checked under the lock.
//  4. Check for existing enrollment (duplicate guard).
//  5. INSERT enrollment.
//  6. INSERT enrollment_history.
//  7. COMMIT.
//
// Any failure inside the transaction triggers ROLLBACK.
func (r *EnrollmentRepo) Enroll(ctx context.Context, req enrollment.EnrollRequest) (*model.Enrollment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin enrollment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	// ── Step 1: Lock the program row and read capacity ────────────────────────
	var capacity int
	err = tx.QueryRow(ctx,
		`SELECT capacity FROM lms.programs WHERE id = $1 FOR UPDATE`,
		req.ProgramID,
	).Scan(&capacity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "program", ID: req.ProgramID}
		}
		return nil, err
	}

	// ── Step 2: Re-check confirmed count under the lock ───────────────────────
	var confirmedCount int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM lms.enrollments WHERE program_id = $1 AND status = 'confirmed'`,
		req.ProgramID,
	).Scan(&confirmedCount); err != nil {
		return nil, err
	}
	if confirmedCount >= capacity {
		return nil, &apperr.Conflict{
			Resource: "enrollment",
			Message:  fmt.Sprintf("program is full (%d/%d seats taken)", confirmedCount, capacity),
		}
	}
	remainingSeats := capacity - confirmedCount

	// ── Step 3: Duplicate enrollment guard ────────────────────────────────────
	var existingID string
	err = tx.QueryRow(ctx,
		`SELECT id::text FROM lms.enrollments WHERE program_id = $1 AND reader_id = $2`,
		req.ProgramID, req.ReaderID,
	).Scan(&existingID)
	if err == nil {
		// Row found — duplicate.
		return nil, &apperr.Conflict{
			Resource: "enrollment",
			Message:  fmt.Sprintf("reader is already enrolled (enrollment %s)", existingID),
		}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// ── Step 4: Schedule conflict guard ───────────────────────────────────────
	// Check whether the reader has another confirmed/pending enrollment that
	// overlaps with this program's time slot.
	var conflictingProgramID string
	err = tx.QueryRow(ctx, `
		SELECT e.program_id::text
		FROM   lms.enrollments e
		JOIN   lms.programs    p ON p.id = e.program_id
		WHERE  e.reader_id = $1
		  AND  e.status IN ('confirmed', 'pending')
		  AND  p.id     <> $2
		  AND  p.status NOT IN ('cancelled', 'completed')
		  AND  p.starts_at < (SELECT ends_at   FROM lms.programs WHERE id = $2)
		  AND  p.ends_at   > (SELECT starts_at FROM lms.programs WHERE id = $2)
		LIMIT 1`,
		req.ReaderID, req.ProgramID,
	).Scan(&conflictingProgramID)
	if err == nil {
		return nil, &apperr.Conflict{
			Resource: "enrollment",
			Message:  fmt.Sprintf("schedule conflict with program %s", conflictingProgramID),
		}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// ── Step 5: Insert enrollment ─────────────────────────────────────────────
	channel := req.EnrollmentChannel
	if channel == "" {
		channel = "any"
	}
	e := &model.Enrollment{}
	err = tx.QueryRow(ctx, `
		INSERT INTO lms.enrollments
		    (program_id, reader_id, branch_id, status, enrollment_channel, enrolled_by)
		VALUES ($1, $2, $3, 'confirmed', $4, $5::uuid)
		RETURNING `+enrollmentSelectCols,
		req.ProgramID, req.ReaderID, req.BranchID, channel, nullableUUID(req.EnrolledByUserID),
	).Scan(
		&e.ID, &e.ProgramID, &e.ReaderID, &e.BranchID,
		&e.Status, &e.EnrollmentChannel, &e.WaitlistPosition,
		&e.EnrolledAt, &e.UpdatedAt, &e.EnrolledBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert enrollment: %w", err)
	}

	// ── Step 6: Insert history record ─────────────────────────────────────────
	if _, err := tx.Exec(ctx, `
		INSERT INTO lms.enrollment_history
		    (enrollment_id, previous_status, new_status, changed_by, reason, workstation_id)
		VALUES ($1, 'none', 'confirmed', $2::uuid, 'initial enrollment', $3)`,
		e.ID, nullableUUID(req.EnrolledByUserID), nullableStr(req.WorkstationID),
	); err != nil {
		return nil, fmt.Errorf("insert enrollment history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit enrollment: %w", err)
	}

	// Attach computed remaining seats for immediate feedback.
	remaining := remainingSeats - 1
	e.RemainingSeats = &remaining
	return e, nil
}

// Drop cancels an enrollment and records the history change.
func (r *EnrollmentRepo) Drop(ctx context.Context, enrollmentID, readerID, branchID, reason, changedByUserID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin drop transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Fetch and lock the enrollment.
	e := &model.Enrollment{}
	err = tx.QueryRow(ctx, `
		SELECT `+enrollmentSelectCols+`
		FROM lms.enrollments
		WHERE id = $1 AND reader_id = $2 AND branch_id = $3
		FOR UPDATE`,
		enrollmentID, readerID, branchID,
	).Scan(
		&e.ID, &e.ProgramID, &e.ReaderID, &e.BranchID,
		&e.Status, &e.EnrollmentChannel, &e.WaitlistPosition,
		&e.EnrolledAt, &e.UpdatedAt, &e.EnrolledBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &apperr.NotFound{Resource: "enrollment", ID: enrollmentID}
		}
		return err
	}
	if e.Status == "cancelled" {
		return &apperr.Conflict{Resource: "enrollment", Message: "enrollment is already cancelled"}
	}

	prevStatus := e.Status

	// Update enrollment status.
	if _, err := tx.Exec(ctx, `
		UPDATE lms.enrollments
		SET status='cancelled', updated_at=NOW()
		WHERE id=$1`, enrollmentID,
	); err != nil {
		return err
	}

	// Record history.
	if _, err := tx.Exec(ctx, `
		INSERT INTO lms.enrollment_history
		    (enrollment_id, previous_status, new_status, changed_by, reason)
		VALUES ($1, $2, 'cancelled', $3::uuid, $4)`,
		enrollmentID, prevStatus, nullableUUID(changedByUserID), nullableStr(reason),
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetByID returns an enrollment scoped to branchID.
func (r *EnrollmentRepo) GetByID(ctx context.Context, id, branchID string) (*model.Enrollment, error) {
	q := `SELECT ` + enrollmentSelectCols + ` FROM lms.enrollments WHERE id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND branch_id = $2"
		args = append(args, branchID)
	}
	e, err := scanEnrollment(r.pool.QueryRow(ctx, q, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "enrollment", ID: id}
		}
		return nil, err
	}
	return e, nil
}

// ListByProgram returns paginated enrollments for a program.
func (r *EnrollmentRepo) ListByProgram(ctx context.Context, programID, branchID string, p model.Pagination) (model.PageResult[*model.Enrollment], error) {
	where := "program_id = $1"
	args := []any{programID}
	if branchID != "" {
		where += " AND branch_id = $2"
		args = append(args, branchID)
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM lms.enrollments WHERE "+where, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.Enrollment]{}, err
	}

	args = append(args, p.Limit(), p.Offset())
	n := len(args)
	rows, err := r.pool.Query(ctx,
		`SELECT `+enrollmentSelectCols+` FROM lms.enrollments WHERE `+where+
			fmt.Sprintf(` ORDER BY enrolled_at DESC LIMIT $%d OFFSET $%d`, n-1, n),
		args...)
	if err != nil {
		return model.PageResult[*model.Enrollment]{}, err
	}
	defer rows.Close()

	return scanEnrollments(rows, total, p)
}

// ListByReader returns paginated enrollments for a reader.
func (r *EnrollmentRepo) ListByReader(ctx context.Context, readerID, branchID string, p model.Pagination) (model.PageResult[*model.Enrollment], error) {
	where := "reader_id = $1"
	args := []any{readerID}
	if branchID != "" {
		where += " AND branch_id = $2"
		args = append(args, branchID)
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM lms.enrollments WHERE "+where, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.Enrollment]{}, err
	}

	args = append(args, p.Limit(), p.Offset())
	n := len(args)
	rows, err := r.pool.Query(ctx,
		`SELECT `+enrollmentSelectCols+` FROM lms.enrollments WHERE `+where+
			fmt.Sprintf(` ORDER BY enrolled_at DESC LIMIT $%d OFFSET $%d`, n-1, n),
		args...)
	if err != nil {
		return model.PageResult[*model.Enrollment]{}, err
	}
	defer rows.Close()

	return scanEnrollments(rows, total, p)
}

// GetHistory returns the ordered status-change log for an enrollment.
func (r *EnrollmentRepo) GetHistory(ctx context.Context, enrollmentID string) ([]*model.EnrollmentHistory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, enrollment_id::text, previous_status, new_status,
		       changed_by::text, reason, changed_at, workstation_id
		FROM lms.enrollment_history
		WHERE enrollment_id = $1
		ORDER BY changed_at`, enrollmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hist []*model.EnrollmentHistory
	for rows.Next() {
		h := &model.EnrollmentHistory{}
		if err := rows.Scan(
			&h.ID, &h.EnrollmentID, &h.PreviousStatus, &h.NewStatus,
			&h.ChangedBy, &h.Reason, &h.ChangedAt, &h.WorkstationID,
		); err != nil {
			return nil, err
		}
		hist = append(hist, h)
	}
	if hist == nil {
		hist = []*model.EnrollmentHistory{}
	}
	return hist, nil
}

// ConfirmedCount returns the number of confirmed enrollments for a program.
func (r *EnrollmentRepo) ConfirmedCount(ctx context.Context, programID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lms.enrollments WHERE program_id = $1 AND status = 'confirmed'`,
		programID,
	).Scan(&count)
	return count, err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func scanEnrollments(rows pgx.Rows, total int, p model.Pagination) (model.PageResult[*model.Enrollment], error) {
	var items []*model.Enrollment
	for rows.Next() {
		e, err := scanEnrollment(rows)
		if err != nil {
			return model.PageResult[*model.Enrollment]{}, err
		}
		items = append(items, e)
	}
	if items == nil {
		items = []*model.Enrollment{}
	}
	return model.NewPageResult(items, total, p), nil
}

// nullableUUID returns nil if the string is empty, otherwise the string.
// This lets pgx pass NULL for empty UUID fields.
func nullableUUID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nullableStr returns nil for an empty string.
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Ensure compile-time interface satisfaction.
var _ enrollment.Repository = (*EnrollmentRepo)(nil)
