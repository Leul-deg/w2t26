package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/content"
	"lms/internal/model"
)

// ContentRepo implements content.Repository against lms.governed_content.
type ContentRepo struct {
	pool *pgxpool.Pool
}

// NewContentRepo creates a new ContentRepo.
func NewContentRepo(pool *pgxpool.Pool) *ContentRepo {
	return &ContentRepo{pool: pool}
}

const contentSelectCols = `
	id::text, branch_id::text, title, content_type, body,
	file_path, file_name, status, submitted_by::text,
	submitted_at, published_at, archived_at, rejection_reason,
	created_at, updated_at`

func scanContent(row pgx.Row) (*model.GovernedContent, error) {
	c := &model.GovernedContent{}
	err := row.Scan(
		&c.ID, &c.BranchID, &c.Title, &c.ContentType, &c.Body,
		&c.FilePath, &c.FileName, &c.Status, &c.SubmittedBy,
		&c.SubmittedAt, &c.PublishedAt, &c.ArchivedAt, &c.RejectionReason,
		&c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

// Create inserts a new governed content item.
func (r *ContentRepo) Create(ctx context.Context, c *model.GovernedContent) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO lms.governed_content
		    (branch_id, title, content_type, body, file_path, file_name,
		     status, submitted_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+contentSelectCols,
		c.BranchID, c.Title, c.ContentType, c.Body, c.FilePath, c.FileName,
		c.Status, c.SubmittedBy,
	).Scan(
		&c.ID, &c.BranchID, &c.Title, &c.ContentType, &c.Body,
		&c.FilePath, &c.FileName, &c.Status, &c.SubmittedBy,
		&c.SubmittedAt, &c.PublishedAt, &c.ArchivedAt, &c.RejectionReason,
		&c.CreatedAt, &c.UpdatedAt,
	)
}

// GetByID returns a governed content item, optionally scoped to a branch.
func (r *ContentRepo) GetByID(ctx context.Context, id, branchID string) (*model.GovernedContent, error) {
	q := `SELECT ` + contentSelectCols + ` FROM lms.governed_content WHERE id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND branch_id = $2"
		args = append(args, branchID)
	}
	c, err := scanContent(r.pool.QueryRow(ctx, q, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "content_item", ID: id}
		}
		return nil, err
	}
	return c, nil
}

// Update writes all mutable fields of a content item.
func (r *ContentRepo) Update(ctx context.Context, c *model.GovernedContent) error {
	return r.pool.QueryRow(ctx, `
		UPDATE lms.governed_content
		SET title=$2, body=$3, file_name=$4, status=$5,
		    published_at=$6, archived_at=$7, rejection_reason=$8, updated_at=NOW()
		WHERE id=$1
		RETURNING `+contentSelectCols,
		c.ID, c.Title, c.Body, c.FileName, c.Status,
		c.PublishedAt, c.ArchivedAt, c.RejectionReason,
	).Scan(
		&c.ID, &c.BranchID, &c.Title, &c.ContentType, &c.Body,
		&c.FilePath, &c.FileName, &c.Status, &c.SubmittedBy,
		&c.SubmittedAt, &c.PublishedAt, &c.ArchivedAt, &c.RejectionReason,
		&c.CreatedAt, &c.UpdatedAt,
	)
}

// UpdateStatus updates only the status field.
func (r *ContentRepo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.governed_content SET status=$2, updated_at=NOW() WHERE id=$1`,
		id, status)
	return err
}

// List returns a filtered, paginated list of governed content items.
func (r *ContentRepo) List(ctx context.Context, branchID string, f content.ContentFilter, p model.Pagination) (model.PageResult[*model.GovernedContent], error) {
	where := []string{}
	args := []any{}
	idx := 1

	if branchID != "" {
		where = append(where, "branch_id = $"+itoa(idx))
		args = append(args, branchID)
		idx++
	}
	if f.Status != nil {
		where = append(where, "status = $"+itoa(idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.ContentType != nil {
		where = append(where, "content_type = $"+itoa(idx))
		args = append(args, *f.ContentType)
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM lms.governed_content "+whereClause, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.GovernedContent]{}, err
	}

	args = append(args, p.Limit(), p.Offset())
	rows, err := r.pool.Query(ctx,
		`SELECT `+contentSelectCols+` FROM lms.governed_content `+whereClause+
			` ORDER BY created_at DESC LIMIT $`+itoa(idx)+` OFFSET $`+itoa(idx+1),
		args...)
	if err != nil {
		return model.PageResult[*model.GovernedContent]{}, err
	}
	defer rows.Close()

	var items []*model.GovernedContent
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return model.PageResult[*model.GovernedContent]{}, err
		}
		items = append(items, c)
	}
	if items == nil {
		items = []*model.GovernedContent{}
	}
	return model.NewPageResult(items, total, p), nil
}

var _ content.Repository = (*ContentRepo)(nil)
