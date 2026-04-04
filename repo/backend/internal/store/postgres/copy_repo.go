package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/model"
)

// CopyRepo implements copies.Repository.
type CopyRepo struct {
	pool *pgxpool.Pool
}

// NewCopyRepo creates a new CopyRepo.
func NewCopyRepo(pool *pgxpool.Pool) *CopyRepo {
	return &CopyRepo{pool: pool}
}

const copySelectCols = `
	id::text, holding_id::text, branch_id::text, barcode, status_code, condition,
	shelf_location, acquired_at::text, withdrawn_at::text, price_paid, notes,
	created_at, updated_at`

func scanCopy(row pgx.Row) (*model.Copy, error) {
	c := &model.Copy{}
	err := row.Scan(
		&c.ID, &c.HoldingID, &c.BranchID, &c.Barcode, &c.StatusCode, &c.Condition,
		&c.ShelfLocation, &c.AcquiredAt, &c.WithdrawnAt, &c.PricePaid, &c.Notes,
		&c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// Create inserts a new copy. Returns apperr.Conflict on duplicate barcode.
func (r *CopyRepo) Create(ctx context.Context, c *model.Copy) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO lms.copies
		    (holding_id, branch_id, barcode, status_code, condition,
		     shelf_location, acquired_at, withdrawn_at, price_paid, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7::date,$8::date,$9,$10)
		RETURNING `+copySelectCols,
		c.HoldingID, c.BranchID, c.Barcode, c.StatusCode, c.Condition,
		c.ShelfLocation, c.AcquiredAt, c.WithdrawnAt, c.PricePaid, c.Notes,
	).Scan(
		&c.ID, &c.HoldingID, &c.BranchID, &c.Barcode, &c.StatusCode, &c.Condition,
		&c.ShelfLocation, &c.AcquiredAt, &c.WithdrawnAt, &c.PricePaid, &c.Notes,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return &apperr.Conflict{Resource: "copy", Message: "barcode already in use: " + c.Barcode}
		}
		return err
	}
	return nil
}

// GetByID returns the copy optionally scoped to branchID.
func (r *CopyRepo) GetByID(ctx context.Context, id, branchID string) (*model.Copy, error) {
	q := `SELECT ` + copySelectCols + ` FROM lms.copies WHERE id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND branch_id = $2"
		args = append(args, branchID)
	}
	c, err := scanCopy(r.pool.QueryRow(ctx, q, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "copy", ID: id}
		}
		return nil, err
	}
	return c, nil
}

// GetByBarcode looks up a copy by its barcode.
// When branchID is non-empty the result is limited to that branch.
// An empty branchID performs a global lookup (used by stocktake to detect misplaced copies).
func (r *CopyRepo) GetByBarcode(ctx context.Context, barcode, branchID string) (*model.Copy, error) {
	var row pgx.Row
	if branchID != "" {
		row = r.pool.QueryRow(ctx,
			`SELECT `+copySelectCols+` FROM lms.copies WHERE barcode = $1 AND branch_id = $2`,
			barcode, branchID)
	} else {
		row = r.pool.QueryRow(ctx,
			`SELECT `+copySelectCols+` FROM lms.copies WHERE barcode = $1`, barcode)
	}
	c, err := scanCopy(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "copy", ID: barcode}
		}
		return nil, err
	}
	return c, nil
}

// Update persists metadata changes (shelf_location, condition, notes, etc.).
func (r *CopyRepo) Update(ctx context.Context, c *model.Copy) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE lms.copies
		SET condition=$1, shelf_location=$2, acquired_at=$3::date,
		    price_paid=$4, notes=$5, updated_at=NOW()
		WHERE id=$6`,
		c.Condition, c.ShelfLocation, c.AcquiredAt, c.PricePaid, c.Notes, c.ID,
	)
	return err
}

// UpdateStatus changes only the status_code of a copy.
func (r *CopyRepo) UpdateStatus(ctx context.Context, id, statusCode string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE lms.copies SET status_code=$1, updated_at=NOW() WHERE id=$2`,
		statusCode, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return &apperr.NotFound{Resource: "copy", ID: id}
	}
	return nil
}

// List returns copies for a holding, optionally filtered by branch.
func (r *CopyRepo) List(ctx context.Context, holdingID, branchID string, p model.Pagination) (model.PageResult[*model.Copy], error) {
	var (
		countQ, dataQ string
		args, dArgs   []any
	)
	if branchID != "" {
		countQ = `SELECT COUNT(*) FROM lms.copies WHERE holding_id=$1 AND branch_id=$2`
		dataQ = fmt.Sprintf(`SELECT `+copySelectCols+
			` FROM lms.copies WHERE holding_id=$1 AND branch_id=$2 ORDER BY barcode LIMIT $3 OFFSET $4`)
		args = []any{holdingID, branchID}
		dArgs = []any{holdingID, branchID, p.Limit(), p.Offset()}
	} else {
		countQ = `SELECT COUNT(*) FROM lms.copies WHERE holding_id=$1`
		dataQ = fmt.Sprintf(`SELECT ` + copySelectCols +
			` FROM lms.copies WHERE holding_id=$1 ORDER BY barcode LIMIT $2 OFFSET $3`)
		args = []any{holdingID}
		dArgs = []any{holdingID, p.Limit(), p.Offset()}
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return model.PageResult[*model.Copy]{}, err
	}

	rows, err := r.pool.Query(ctx, dataQ, dArgs...)
	if err != nil {
		return model.PageResult[*model.Copy]{}, err
	}
	defer rows.Close()

	var items []*model.Copy
	for rows.Next() {
		c, err := scanCopy(rows)
		if err != nil {
			return model.PageResult[*model.Copy]{}, err
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return model.PageResult[*model.Copy]{}, err
	}
	return model.NewPageResult(items, total, p), nil
}

// ListStatuses returns all copy status lookup values.
func (r *CopyRepo) ListStatuses(ctx context.Context) ([]*model.CopyStatus, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT code, description, is_borrowable FROM lms.copy_statuses ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.CopyStatus
	for rows.Next() {
		s := &model.CopyStatus{}
		if err := rows.Scan(&s.Code, &s.Description, &s.IsBorrowable); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
