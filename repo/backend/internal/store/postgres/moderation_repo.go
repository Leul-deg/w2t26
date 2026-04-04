package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/moderation"
	"lms/internal/model"
)

// ModerationRepo implements moderation.Repository against lms.moderation_items.
type ModerationRepo struct {
	pool *pgxpool.Pool
}

// NewModerationRepo creates a new ModerationRepo.
func NewModerationRepo(pool *pgxpool.Pool) *ModerationRepo {
	return &ModerationRepo{pool: pool}
}

const moderationSelectCols = `
	mi.id::text, mi.content_id::text, mi.assigned_to::text,
	mi.status, mi.decision, mi.decision_reason, mi.decided_by::text, mi.decided_at,
	mi.created_at, mi.updated_at`

func scanModerationItem(row pgx.Row) (*model.ModerationItem, error) {
	m := &model.ModerationItem{}
	err := row.Scan(
		&m.ID, &m.ContentID, &m.AssignedTo,
		&m.Status, &m.Decision, &m.DecisionReason, &m.DecidedBy, &m.DecidedAt,
		&m.CreatedAt, &m.UpdatedAt,
	)
	return m, err
}

// Create inserts a new moderation item.
func (r *ModerationRepo) Create(ctx context.Context, item *model.ModerationItem) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO lms.moderation_items (content_id, status)
		VALUES ($1, $2)
		RETURNING `+moderationSelectCols,
		item.ContentID, item.Status,
	).Scan(
		&item.ID, &item.ContentID, &item.AssignedTo,
		&item.Status, &item.Decision, &item.DecisionReason, &item.DecidedBy, &item.DecidedAt,
		&item.CreatedAt, &item.UpdatedAt,
	)
}

// GetByID returns a moderation item by ID, optionally scoped to a content branch.
func (r *ModerationRepo) GetByID(ctx context.Context, id, branchID string) (*model.ModerationItem, error) {
	q := `SELECT ` + moderationSelectCols + `
		FROM lms.moderation_items mi
		JOIN lms.governed_content gc ON gc.id = mi.content_id
		WHERE mi.id = $1`
	args := []any{id}
	if branchID != "" {
		q += ` AND gc.branch_id = $2`
		args = append(args, branchID)
	}
	m, err := scanModerationItem(r.pool.QueryRow(ctx, q, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "moderation_item", ID: id}
		}
		return nil, err
	}
	return m, nil
}

// GetByContentID returns the most recent non-decided item for a content item.
func (r *ModerationRepo) GetByContentID(ctx context.Context, contentID string) (*model.ModerationItem, error) {
	m, err := scanModerationItem(r.pool.QueryRow(ctx,
		`SELECT `+moderationSelectCols+`
		 FROM lms.moderation_items
		 WHERE content_id = $1 AND status <> 'decided'
		 ORDER BY created_at DESC LIMIT 1`, contentID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "moderation_item", ID: contentID}
		}
		return nil, err
	}
	return m, nil
}

// Decide records a moderation decision or assignment.
// When decision is empty, only assigned_to and status (in_review) are updated.
// When decision is set, the item is moved to decided.
func (r *ModerationRepo) Decide(ctx context.Context, id, decision, reason, decidedByUserID string) error {
	if decision == "" {
		// Assign-only: set assigned_to and status=in_review.
		_, err := r.pool.Exec(ctx, `
			UPDATE lms.moderation_items
			SET assigned_to=$2, status='in_review', updated_at=NOW()
			WHERE id=$1 AND status <> 'decided'`,
			id, decidedByUserID)
		return err
	}

	// Full decision: update decision fields.
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE lms.moderation_items
		SET status='decided', decision=$2, decision_reason=$3,
		    decided_by=$4, decided_at=$5, updated_at=NOW()
		WHERE id=$1 AND status <> 'decided'`,
		id, decision, reason, decidedByUserID, now)
	return err
}

// List returns a filtered, paginated list of moderation items.
func (r *ModerationRepo) List(ctx context.Context, branchID string, f moderation.ModerationFilter, p model.Pagination) (model.PageResult[*model.ModerationItem], error) {
	where := []string{}
	args := []any{}
	idx := 1

	fromClause := ` FROM lms.moderation_items mi
		JOIN lms.governed_content gc ON gc.id = mi.content_id`

	if branchID != "" {
		where = append(where, "gc.branch_id = $"+itoa(idx))
		args = append(args, branchID)
		idx++
	}

	if f.Status != nil {
		where = append(where, "mi.status = $"+itoa(idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.AssignedTo != nil {
		where = append(where, "mi.assigned_to = $"+itoa(idx))
		args = append(args, *f.AssignedTo)
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*)"+fromClause+" "+whereClause, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.ModerationItem]{}, err
	}

	args = append(args, p.Limit(), p.Offset())
	rows, err := r.pool.Query(ctx,
		`SELECT `+moderationSelectCols+fromClause+` `+whereClause+
			` ORDER BY mi.created_at ASC LIMIT $`+itoa(idx)+` OFFSET $`+itoa(idx+1),
		args...)
	if err != nil {
		return model.PageResult[*model.ModerationItem]{}, err
	}
	defer rows.Close()

	var items []*model.ModerationItem
	for rows.Next() {
		m, err := scanModerationItem(rows)
		if err != nil {
			return model.PageResult[*model.ModerationItem]{}, err
		}
		items = append(items, m)
	}
	if items == nil {
		items = []*model.ModerationItem{}
	}
	return model.NewPageResult(items, total, p), nil
}

var _ moderation.Repository = (*ModerationRepo)(nil)
