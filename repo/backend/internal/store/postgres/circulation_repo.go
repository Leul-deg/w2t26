package postgres

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/circulation"
	"lms/internal/model"
)

// CirculationRepo implements circulation.Repository.
type CirculationRepo struct {
	pool *pgxpool.Pool
}

// NewCirculationRepo creates a new CirculationRepo.
func NewCirculationRepo(pool *pgxpool.Pool) *CirculationRepo {
	return &CirculationRepo{pool: pool}
}

var _ circulation.Repository = (*CirculationRepo)(nil)

const circulationSelectCols = `
	id::text, copy_id::text, reader_id::text, branch_id::text,
	event_type, due_date::text, returned_at, destination_branch_id::text,
	performed_by::text, workstation_id, notes, created_at`

func scanCirculationEvent(row pgx.Row) (*model.CirculationEvent, error) {
	e := &model.CirculationEvent{}
	err := row.Scan(
		&e.ID, &e.CopyID, &e.ReaderID, &e.BranchID,
		&e.EventType, &e.DueDate, &e.ReturnedAt, &e.DestinationBranchID,
		&e.PerformedBy, &e.WorkstationID, &e.Notes, &e.CreatedAt,
	)
	return e, err
}

// Checkout atomically inserts a checkout event and sets copy status to "checked_out".
//
// Transaction sequence:
//  1. Lock the copy row (FOR UPDATE) and verify status is "available".
//  2. Insert the circulation_event row.
//  3. Update copies.status_code to "checked_out".
func (r *CirculationRepo) Checkout(ctx context.Context, e *model.CirculationEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Lock and verify the copy is available, scoped to the event's branch for
	// defence-in-depth: even if the service layer's branch check were bypassed,
	// the repo would refuse to lock a copy outside the stated branch.
	var statusCode string
	err = tx.QueryRow(ctx, `
		SELECT status_code FROM lms.copies WHERE id = $1 AND branch_id = $2 FOR UPDATE`,
		e.CopyID, e.BranchID,
	).Scan(&statusCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return &apperr.NotFound{Resource: "copy", ID: e.CopyID}
	}
	if err != nil {
		return err
	}
	if statusCode != "available" {
		return &apperr.Conflict{Resource: "copy", Message: "copy is not available for checkout (status: " + statusCode + ")"}
	}

	// Insert the circulation event.
	err = tx.QueryRow(ctx, `
		INSERT INTO lms.circulation_events
			(copy_id, reader_id, branch_id, event_type, due_date, performed_by, workstation_id, notes)
		VALUES ($1, $2, $3, $4, $5::date, $6, $7, $8)
		RETURNING `+circulationSelectCols,
		e.CopyID, e.ReaderID, e.BranchID, e.EventType,
		e.DueDate, e.PerformedBy, e.WorkstationID, e.Notes,
	).Scan(
		&e.ID, &e.CopyID, &e.ReaderID, &e.BranchID,
		&e.EventType, &e.DueDate, &e.ReturnedAt, &e.DestinationBranchID,
		&e.PerformedBy, &e.WorkstationID, &e.Notes, &e.CreatedAt,
	)
	if err != nil {
		return err
	}

	// Update copy status.
	if _, err := tx.Exec(ctx, `
		UPDATE lms.copies SET status_code = 'checked_out', updated_at = NOW()
		WHERE id = $1`, e.CopyID,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Return atomically inserts a return event and resets copy status to "available".
//
// Transaction sequence:
//  1. Lock the copy row (FOR UPDATE) and verify status is "checked_out".
//  2. Insert the circulation_event row.
//  3. Update copies.status_code to "available".
func (r *CirculationRepo) Return(ctx context.Context, e *model.CirculationEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Lock and verify the copy is checked out, scoped to the event's branch for
	// defence-in-depth (see Checkout for rationale).
	var statusCode string
	err = tx.QueryRow(ctx, `
		SELECT status_code FROM lms.copies WHERE id = $1 AND branch_id = $2 FOR UPDATE`,
		e.CopyID, e.BranchID,
	).Scan(&statusCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return &apperr.NotFound{Resource: "copy", ID: e.CopyID}
	}
	if err != nil {
		return err
	}
	if statusCode != "checked_out" {
		return &apperr.Conflict{Resource: "copy", Message: "copy is not checked out (status: " + statusCode + ")"}
	}

	// Insert the return event.
	err = tx.QueryRow(ctx, `
		INSERT INTO lms.circulation_events
			(copy_id, reader_id, branch_id, event_type, returned_at, performed_by, workstation_id, notes)
		VALUES ($1, $2, $3, $4, NOW(), $5, $6, $7)
		RETURNING `+circulationSelectCols,
		e.CopyID, e.ReaderID, e.BranchID, e.EventType,
		e.PerformedBy, e.WorkstationID, e.Notes,
	).Scan(
		&e.ID, &e.CopyID, &e.ReaderID, &e.BranchID,
		&e.EventType, &e.DueDate, &e.ReturnedAt, &e.DestinationBranchID,
		&e.PerformedBy, &e.WorkstationID, &e.Notes, &e.CreatedAt,
	)
	if err != nil {
		return err
	}

	// Mark the active checkout row as returned so GetActiveCheckout returns 404
	// after the copy is back. The returned_at field on a checkout row signals that
	// the loan has been closed; without this update, GetActiveCheckout would still
	// find the row (event_type='checkout' AND returned_at IS NULL).
	if _, err := tx.Exec(ctx, `
		UPDATE lms.circulation_events
		   SET returned_at = NOW()
		 WHERE copy_id = $1 AND event_type = 'checkout' AND returned_at IS NULL`,
		e.CopyID,
	); err != nil {
		return err
	}

	// Reset copy status.
	if _, err := tx.Exec(ctx, `
		UPDATE lms.copies SET status_code = 'available', updated_at = NOW()
		WHERE id = $1`, e.CopyID,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Record inserts a raw circulation event without touching copy status.
func (r *CirculationRepo) Record(ctx context.Context, e *model.CirculationEvent) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO lms.circulation_events
			(copy_id, reader_id, branch_id, event_type, due_date, performed_by, workstation_id, notes)
		VALUES ($1, $2, $3, $4, $5::date, $6, $7, $8)
		RETURNING `+circulationSelectCols,
		e.CopyID, e.ReaderID, e.BranchID, e.EventType,
		e.DueDate, e.PerformedBy, e.WorkstationID, e.Notes,
	).Scan(
		&e.ID, &e.CopyID, &e.ReaderID, &e.BranchID,
		&e.EventType, &e.DueDate, &e.ReturnedAt, &e.DestinationBranchID,
		&e.PerformedBy, &e.WorkstationID, &e.Notes, &e.CreatedAt,
	)
	return err
}

// GetActiveCheckout returns the most recent unresolved checkout for a copy.
func (r *CirculationRepo) GetActiveCheckout(ctx context.Context, copyID string) (*model.CirculationEvent, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+circulationSelectCols+`
		FROM lms.circulation_events
		WHERE copy_id = $1
		  AND event_type = 'checkout'
		  AND returned_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1`,
		copyID,
	)
	e, err := scanCirculationEvent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apperr.NotFound{Resource: "active checkout", ID: copyID}
	}
	return e, err
}

// ListByBranch returns paginated circulation events for a branch.
// An empty branchID returns events across all branches (administrator scope).
func (r *CirculationRepo) ListByBranch(ctx context.Context, branchID string, f circulation.CirculationFilter, p model.Pagination) (model.PageResult[*model.CirculationEvent], error) {
	var args []any
	var conds []string
	idx := 1
	if branchID != "" {
		conds = append(conds, "branch_id = $"+strconv.Itoa(idx))
		args = append(args, branchID)
		idx++
	}

	if f.EventType != nil {
		conds = append(conds, "event_type = $"+strconv.Itoa(idx))
		args = append(args, *f.EventType)
		idx++
	}
	if f.ReaderID != nil {
		conds = append(conds, "reader_id = $"+strconv.Itoa(idx))
		args = append(args, *f.ReaderID)
		idx++
	}
	if f.CopyID != nil {
		conds = append(conds, "copy_id = $"+strconv.Itoa(idx))
		args = append(args, *f.CopyID)
		idx++
	}

	var where string
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lms.circulation_events `+where, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.CirculationEvent]{}, err
	}

	offset := (p.Page - 1) * p.PerPage
	listArgs := append(args, p.PerPage, offset)
	rows, err := r.pool.Query(ctx,
		`SELECT `+circulationSelectCols+` FROM lms.circulation_events `+where+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(idx)+` OFFSET $`+strconv.Itoa(idx+1),
		listArgs...,
	)
	if err != nil {
		return model.PageResult[*model.CirculationEvent]{}, err
	}
	defer rows.Close()

	items, err := scanCirculationRows(rows)
	if err != nil {
		return model.PageResult[*model.CirculationEvent]{}, err
	}
	return model.NewPageResult(items, total, p), nil
}

// ListByCopy returns paginated circulation events for a specific copy.
func (r *CirculationRepo) ListByCopy(ctx context.Context, copyID, branchID string, p model.Pagination) (model.PageResult[*model.CirculationEvent], error) {
	where := "WHERE copy_id = $1"
	args := []any{copyID}
	if branchID != "" {
		where += " AND branch_id = $2"
		args = append(args, branchID)
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lms.circulation_events `+where, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.CirculationEvent]{}, err
	}

	offset := (p.Page - 1) * p.PerPage
	nextIdx := len(args) + 1
	listArgs := append(args, p.PerPage, offset)
	rows, err := r.pool.Query(ctx,
		`SELECT `+circulationSelectCols+` FROM lms.circulation_events `+where+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(nextIdx)+` OFFSET $`+strconv.Itoa(nextIdx+1),
		listArgs...,
	)
	if err != nil {
		return model.PageResult[*model.CirculationEvent]{}, err
	}
	defer rows.Close()

	items, err := scanCirculationRows(rows)
	if err != nil {
		return model.PageResult[*model.CirculationEvent]{}, err
	}
	return model.NewPageResult(items, total, p), nil
}

// ListByReader returns paginated circulation events for a specific reader.
func (r *CirculationRepo) ListByReader(ctx context.Context, readerID, branchID string, p model.Pagination) (model.PageResult[*model.CirculationEvent], error) {
	where := "WHERE reader_id = $1"
	args := []any{readerID}
	if branchID != "" {
		where += " AND branch_id = $2"
		args = append(args, branchID)
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lms.circulation_events `+where, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.CirculationEvent]{}, err
	}

	offset := (p.Page - 1) * p.PerPage
	nextIdx := len(args) + 1
	listArgs := append(args, p.PerPage, offset)
	rows, err := r.pool.Query(ctx,
		`SELECT `+circulationSelectCols+` FROM lms.circulation_events `+where+
			` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(nextIdx)+` OFFSET $`+strconv.Itoa(nextIdx+1),
		listArgs...,
	)
	if err != nil {
		return model.PageResult[*model.CirculationEvent]{}, err
	}
	defer rows.Close()

	items, err := scanCirculationRows(rows)
	if err != nil {
		return model.PageResult[*model.CirculationEvent]{}, err
	}
	return model.NewPageResult(items, total, p), nil
}

func scanCirculationRows(rows pgx.Rows) ([]*model.CirculationEvent, error) {
	var items []*model.CirculationEvent
	for rows.Next() {
		e, err := scanCirculationEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if items == nil {
		items = []*model.CirculationEvent{}
	}
	return items, rows.Err()
}

