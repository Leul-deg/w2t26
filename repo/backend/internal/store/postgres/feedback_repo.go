package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/feedback"
	"lms/internal/model"
)

// FeedbackRepo implements feedback.Repository against lms.feedback and lms.feedback_tags.
type FeedbackRepo struct {
	pool *pgxpool.Pool
}

// NewFeedbackRepo creates a new FeedbackRepo.
func NewFeedbackRepo(pool *pgxpool.Pool) *FeedbackRepo {
	return &FeedbackRepo{pool: pool}
}

// Create inserts a new feedback item and its tag mappings.
func (r *FeedbackRepo) Create(ctx context.Context, f *model.Feedback, tagIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	err = tx.QueryRow(ctx, `
		INSERT INTO lms.feedback
		    (branch_id, reader_id, target_type, target_id, rating, comment, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id::text, submitted_at`,
		f.BranchID, f.ReaderID, f.TargetType, f.TargetID, f.Rating, f.Comment, f.Status,
	).Scan(&f.ID, &f.SubmittedAt)
	if err != nil {
		return err
	}

	// Insert tag mappings.
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO lms.feedback_tag_mappings (feedback_id, tag_id) VALUES ($1,$2)`,
			f.ID, tagID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// GetByID returns a feedback item with its resolved tag names.
func (r *FeedbackRepo) GetByID(ctx context.Context, id, branchID string) (*model.Feedback, error) {
	q := `
		SELECT f.id::text, f.branch_id::text, f.reader_id::text,
		       f.target_type, f.target_id::text,
		       f.rating, f.comment, f.status,
		       f.moderated_by::text, f.moderated_at, f.submitted_at,
		       COALESCE(
		           ARRAY(
		               SELECT ft.name
		               FROM lms.feedback_tag_mappings ftm
		               JOIN lms.feedback_tags ft ON ft.id = ftm.tag_id
		               WHERE ftm.feedback_id = f.id
		               ORDER BY ft.name
		           ),
		           '{}'::text[]
		       ) AS tags
		FROM lms.feedback f
		WHERE f.id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND f.branch_id = $2"
		args = append(args, branchID)
	}

	f := &model.Feedback{}
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&f.ID, &f.BranchID, &f.ReaderID,
		&f.TargetType, &f.TargetID,
		&f.Rating, &f.Comment, &f.Status,
		&f.ModeratedBy, &f.ModeratedAt, &f.SubmittedAt,
		&f.Tags,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "feedback", ID: id}
		}
		return nil, err
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	return f, nil
}

// Moderate updates the status of a feedback item.
func (r *FeedbackRepo) Moderate(ctx context.Context, id, status, moderatedByUserID string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE lms.feedback
		SET status=$2, moderated_by=$3, moderated_at=$4
		WHERE id=$1`,
		id, status, moderatedByUserID, now)
	return err
}

// List returns a paginated list of feedback items with tags resolved.
func (r *FeedbackRepo) List(ctx context.Context, branchID string, f feedback.FeedbackFilter, p model.Pagination) (model.PageResult[*model.Feedback], error) {
	where := []string{}
	args := []any{}
	idx := 1

	if branchID != "" {
		where = append(where, "f.branch_id = $"+itoa(idx))
		args = append(args, branchID)
		idx++
	}
	if f.Status != nil {
		where = append(where, "f.status = $"+itoa(idx))
		args = append(args, *f.Status)
		idx++
	}
	if f.TargetType != nil {
		where = append(where, "f.target_type = $"+itoa(idx))
		args = append(args, *f.TargetType)
		idx++
	}
	if f.TargetID != nil {
		where = append(where, "f.target_id = $"+itoa(idx))
		args = append(args, *f.TargetID)
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM lms.feedback f "+whereClause, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.Feedback]{}, err
	}

	args = append(args, p.Limit(), p.Offset())
	rows, err := r.pool.Query(ctx, `
		SELECT f.id::text, f.branch_id::text, f.reader_id::text,
		       f.target_type, f.target_id::text,
		       f.rating, f.comment, f.status,
		       f.moderated_by::text, f.moderated_at, f.submitted_at,
		       COALESCE(
		           ARRAY(
		               SELECT ft.name
		               FROM lms.feedback_tag_mappings ftm
		               JOIN lms.feedback_tags ft ON ft.id = ftm.tag_id
		               WHERE ftm.feedback_id = f.id
		               ORDER BY ft.name
		           ),
		           '{}'::text[]
		       ) AS tags
		FROM lms.feedback f
		`+whereClause+
		` ORDER BY f.submitted_at DESC LIMIT $`+itoa(idx)+` OFFSET $`+itoa(idx+1),
		args...)
	if err != nil {
		return model.PageResult[*model.Feedback]{}, err
	}
	defer rows.Close()

	var items []*model.Feedback
	for rows.Next() {
		fb := &model.Feedback{}
		if err := rows.Scan(
			&fb.ID, &fb.BranchID, &fb.ReaderID,
			&fb.TargetType, &fb.TargetID,
			&fb.Rating, &fb.Comment, &fb.Status,
			&fb.ModeratedBy, &fb.ModeratedAt, &fb.SubmittedAt,
			&fb.Tags,
		); err != nil {
			return model.PageResult[*model.Feedback]{}, err
		}
		if fb.Tags == nil {
			fb.Tags = []string{}
		}
		items = append(items, fb)
	}
	if items == nil {
		items = []*model.Feedback{}
	}
	return model.NewPageResult(items, total, p), nil
}

// ListTags returns all active feedback tags.
func (r *FeedbackRepo) ListTags(ctx context.Context) ([]*model.FeedbackTag, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id::text, name, is_active FROM lms.feedback_tags WHERE is_active = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*model.FeedbackTag
	for rows.Next() {
		t := &model.FeedbackTag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.IsActive); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []*model.FeedbackTag{}
	}
	return tags, nil
}

var _ feedback.Repository = (*FeedbackRepo)(nil)
