package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/appeals"
	"lms/internal/model"
)

// AppealsRepo implements appeals.Repository against lms.appeals and lms.appeal_arbitrations.
type AppealsRepo struct {
	pool *pgxpool.Pool
}

// NewAppealsRepo creates a new AppealsRepo.
func NewAppealsRepo(pool *pgxpool.Pool) *AppealsRepo {
	return &AppealsRepo{pool: pool}
}

const appealSelectCols = `
	id::text, branch_id::text, reader_id::text,
	appeal_type, target_type, target_id::text,
	reason, status, submitted_at, updated_at`

func scanAppeal(row pgx.Row) (*model.Appeal, error) {
	a := &model.Appeal{}
	err := row.Scan(
		&a.ID, &a.BranchID, &a.ReaderID,
		&a.AppealType, &a.TargetType, &a.TargetID,
		&a.Reason, &a.Status, &a.SubmittedAt, &a.UpdatedAt,
	)
	return a, err
}

// Create inserts a new appeal.
func (r *AppealsRepo) Create(ctx context.Context, a *model.Appeal) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO lms.appeals
		    (branch_id, reader_id, appeal_type, target_type, target_id, reason, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+appealSelectCols,
		a.BranchID, a.ReaderID, a.AppealType, a.TargetType, a.TargetID, a.Reason, a.Status,
	).Scan(
		&a.ID, &a.BranchID, &a.ReaderID,
		&a.AppealType, &a.TargetType, &a.TargetID,
		&a.Reason, &a.Status, &a.SubmittedAt, &a.UpdatedAt,
	)
}

// GetByID returns an appeal, optionally scoped to a branch.
func (r *AppealsRepo) GetByID(ctx context.Context, id, branchID string) (*model.Appeal, error) {
	q := `SELECT ` + appealSelectCols + ` FROM lms.appeals WHERE id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND branch_id = $2"
		args = append(args, branchID)
	}
	a, err := scanAppeal(r.pool.QueryRow(ctx, q, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "appeal", ID: id}
		}
		return nil, err
	}
	return a, nil
}

// UpdateStatus updates the appeal status.
func (r *AppealsRepo) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.appeals SET status=$2, updated_at=NOW() WHERE id=$1`,
		id, status)
	return err
}

// List returns a filtered, paginated list of appeals.
func (r *AppealsRepo) List(ctx context.Context, branchID string, f appeals.AppealFilter, p model.Pagination) (model.PageResult[*model.Appeal], error) {
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
	if f.AppealType != nil {
		where = append(where, "appeal_type = $"+itoa(idx))
		args = append(args, *f.AppealType)
		idx++
	}
	if f.ReaderID != nil {
		where = append(where, "reader_id = $"+itoa(idx))
		args = append(args, *f.ReaderID)
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM lms.appeals "+whereClause, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.Appeal]{}, err
	}

	args = append(args, p.Limit(), p.Offset())
	rows, err := r.pool.Query(ctx,
		`SELECT `+appealSelectCols+` FROM lms.appeals `+whereClause+
			` ORDER BY submitted_at DESC LIMIT $`+itoa(idx)+` OFFSET $`+itoa(idx+1),
		args...)
	if err != nil {
		return model.PageResult[*model.Appeal]{}, err
	}
	defer rows.Close()

	var items []*model.Appeal
	for rows.Next() {
		a, err := scanAppeal(rows)
		if err != nil {
			return model.PageResult[*model.Appeal]{}, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []*model.Appeal{}
	}
	return model.NewPageResult(items, total, p), nil
}

// Arbitrate inserts an appeal arbitration record.
func (r *AppealsRepo) Arbitrate(ctx context.Context, arb *model.AppealArbitration) error {
	beforeJSON, _ := json.Marshal(arb.BeforeState)
	afterJSON, _ := json.Marshal(arb.AfterState)

	return r.pool.QueryRow(ctx, `
		INSERT INTO lms.appeal_arbitrations
		    (appeal_id, arbitrator_id, decision, decision_notes, before_state, after_state)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id::text, decided_at`,
		arb.AppealID, arb.ArbitratorID, arb.Decision, arb.DecisionNotes,
		beforeJSON, afterJSON,
	).Scan(&arb.ID, &arb.DecidedAt)
}

// GetArbitration returns the arbitration record for an appeal (if any).
func (r *AppealsRepo) GetArbitration(ctx context.Context, appealID string) (*model.AppealArbitration, error) {
	arb := &model.AppealArbitration{}
	var beforeRaw, afterRaw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, appeal_id::text, arbitrator_id::text,
		       decision, decision_notes, before_state, after_state, decided_at
		FROM lms.appeal_arbitrations
		WHERE appeal_id = $1
		ORDER BY decided_at DESC LIMIT 1`, appealID,
	).Scan(
		&arb.ID, &arb.AppealID, &arb.ArbitratorID,
		&arb.Decision, &arb.DecisionNotes, &beforeRaw, &afterRaw, &arb.DecidedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "arbitration", ID: appealID}
		}
		return nil, err
	}
	if beforeRaw != nil {
		_ = json.Unmarshal(beforeRaw, &arb.BeforeState)
	}
	if afterRaw != nil {
		_ = json.Unmarshal(afterRaw, &arb.AfterState)
	}
	return arb, nil
}

var _ appeals.Repository = (*AppealsRepo)(nil)
