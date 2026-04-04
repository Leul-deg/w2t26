package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/holdings"
	"lms/internal/model"
)

// HoldingRepo implements holdings.Repository.
type HoldingRepo struct {
	pool *pgxpool.Pool
}

// NewHoldingRepo creates a new HoldingRepo.
func NewHoldingRepo(pool *pgxpool.Pool) *HoldingRepo {
	return &HoldingRepo{pool: pool}
}

const holdingSelectCols = `
	id::text, branch_id::text, title, author, isbn, publisher,
	publication_year, category, subcategory, language,
	description, cover_image_path, is_active,
	created_at, updated_at, created_by::text`

func scanHolding(row pgx.Row) (*model.Holding, error) {
	h := &model.Holding{}
	err := row.Scan(
		&h.ID, &h.BranchID, &h.Title, &h.Author, &h.ISBN, &h.Publisher,
		&h.PublicationYear, &h.Category, &h.Subcategory, &h.Language,
		&h.Description, &h.CoverImagePath, &h.IsActive,
		&h.CreatedAt, &h.UpdatedAt, &h.CreatedBy,
	)
	return h, err
}

// Create inserts a new holding.
func (r *HoldingRepo) Create(ctx context.Context, h *model.Holding) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO lms.holdings
		    (branch_id, title, author, isbn, publisher, publication_year,
		     category, subcategory, language, description, cover_image_path, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+holdingSelectCols,
		h.BranchID, h.Title, h.Author, h.ISBN, h.Publisher, h.PublicationYear,
		h.Category, h.Subcategory, h.Language, h.Description, h.CoverImagePath, h.CreatedBy,
	).Scan(
		&h.ID, &h.BranchID, &h.Title, &h.Author, &h.ISBN, &h.Publisher,
		&h.PublicationYear, &h.Category, &h.Subcategory, &h.Language,
		&h.Description, &h.CoverImagePath, &h.IsActive,
		&h.CreatedAt, &h.UpdatedAt, &h.CreatedBy,
	)
	return err
}

// GetByID returns the holding scoped to branchID.
func (r *HoldingRepo) GetByID(ctx context.Context, id, branchID string) (*model.Holding, error) {
	q := `SELECT ` + holdingSelectCols + ` FROM lms.holdings WHERE id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND branch_id = $2"
		args = append(args, branchID)
	}
	h, err := scanHolding(r.pool.QueryRow(ctx, q, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "holding", ID: id}
		}
		return nil, err
	}
	return h, nil
}

// Update persists non-status changes to a holding.
func (r *HoldingRepo) Update(ctx context.Context, h *model.Holding) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE lms.holdings
		SET title=$1, author=$2, isbn=$3, publisher=$4, publication_year=$5,
		    category=$6, subcategory=$7, language=$8, description=$9,
		    cover_image_path=$10, updated_at=NOW()
		WHERE id=$11`,
		h.Title, h.Author, h.ISBN, h.Publisher, h.PublicationYear,
		h.Category, h.Subcategory, h.Language, h.Description,
		h.CoverImagePath, h.ID,
	)
	return err
}

// Deactivate soft-deletes a holding (is_active = false).
func (r *HoldingRepo) Deactivate(ctx context.Context, id, branchID string) error {
	var err error
	var rowsAffected int64
	if branchID != "" {
		tag, execErr := r.pool.Exec(ctx,
			`UPDATE lms.holdings SET is_active=false, updated_at=NOW() WHERE id=$1 AND branch_id=$2`,
			id, branchID)
		err = execErr
		rowsAffected = tag.RowsAffected()
	} else {
		tag, execErr := r.pool.Exec(ctx,
			`UPDATE lms.holdings SET is_active=false, updated_at=NOW() WHERE id=$1`, id)
		err = execErr
		rowsAffected = tag.RowsAffected()
	}
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return &apperr.NotFound{Resource: "holding", ID: id}
	}
	return nil
}

// List returns a paginated, filtered list of holdings.
func (r *HoldingRepo) List(ctx context.Context, branchID string, f holdings.HoldingFilter, p model.Pagination) (model.PageResult[*model.Holding], error) {
	var where []string
	var args []any
	i := 1

	if branchID != "" {
		where = append(where, fmt.Sprintf("branch_id = $%d", i))
		args = append(args, branchID)
		i++
	}
	if f.ISBN != nil {
		where = append(where, fmt.Sprintf("isbn = $%d", i))
		args = append(args, *f.ISBN)
		i++
	}
	if f.Category != nil {
		where = append(where, fmt.Sprintf("category = $%d", i))
		args = append(args, *f.Category)
		i++
	}
	if f.Active != nil {
		where = append(where, fmt.Sprintf("is_active = $%d", i))
		args = append(args, *f.Active)
		i++
	}
	if f.Search != nil && *f.Search != "" {
		pat := "%" + strings.ToLower(*f.Search) + "%"
		where = append(where, fmt.Sprintf("(LOWER(title) LIKE $%d OR LOWER(COALESCE(author,'')) LIKE $%d)", i, i))
		args = append(args, pat)
		i++
	}

	wClause := ""
	if len(where) > 0 {
		wClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM lms.holdings "+wClause, args...).Scan(&total); err != nil {
		return model.PageResult[*model.Holding]{}, err
	}

	dataArgs := append(args, p.Limit(), p.Offset())
	rows, err := r.pool.Query(ctx, fmt.Sprintf(
		`SELECT `+holdingSelectCols+` FROM lms.holdings %s ORDER BY title, id LIMIT $%d OFFSET $%d`,
		wClause, i, i+1,
	), dataArgs...)
	if err != nil {
		return model.PageResult[*model.Holding]{}, err
	}
	defer rows.Close()

	var items []*model.Holding
	for rows.Next() {
		h, err := scanHolding(rows)
		if err != nil {
			return model.PageResult[*model.Holding]{}, err
		}
		items = append(items, h)
	}
	if err := rows.Err(); err != nil {
		return model.PageResult[*model.Holding]{}, err
	}
	return model.NewPageResult(items, total, p), nil
}
