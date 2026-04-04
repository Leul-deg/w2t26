package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/stocktake"
	"lms/internal/model"
)

// StocktakeRepo implements stocktake.Repository.
type StocktakeRepo struct {
	pool *pgxpool.Pool
}

// NewStocktakeRepo creates a new StocktakeRepo.
func NewStocktakeRepo(pool *pgxpool.Pool) *StocktakeRepo {
	return &StocktakeRepo{pool: pool}
}

// CreateSession inserts a new stocktake session.
func (r *StocktakeRepo) CreateSession(ctx context.Context, s *model.StocktakeSession) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO lms.stocktake_sessions (branch_id, name, status, started_by, notes)
		VALUES ($1, $2, 'open', $3, $4)
		RETURNING id::text, status, started_at, closed_at, created_at`,
		s.BranchID, s.Name, s.StartedBy, s.Notes,
	).Scan(&s.ID, &s.Status, &s.StartedAt, &s.ClosedAt, &s.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return &apperr.Conflict{Resource: "stocktake_session", Message: "branch already has an active stocktake session"}
		}
		return err
	}
	return nil
}

// GetSession returns the session scoped to branchID.
func (r *StocktakeRepo) GetSession(ctx context.Context, id, branchID string) (*model.StocktakeSession, error) {
	q := `
		SELECT id::text, branch_id::text, name, status,
		       started_at, closed_at, started_by::text, closed_by::text, notes, created_at
		FROM lms.stocktake_sessions
		WHERE id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND branch_id = $2"
		args = append(args, branchID)
	}

	s := &model.StocktakeSession{}
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&s.ID, &s.BranchID, &s.Name, &s.Status,
		&s.StartedAt, &s.ClosedAt, &s.StartedBy, &s.ClosedBy, &s.Notes, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "stocktake_session", ID: id}
		}
		return nil, err
	}
	return s, nil
}

// UpdateSessionStatus transitions the session to closed or cancelled.
func (r *StocktakeRepo) UpdateSessionStatus(ctx context.Context, id, status, closedByUserID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE lms.stocktake_sessions
		SET status=$1, closed_at=NOW(), closed_by=$2::uuid
		WHERE id=$3 AND status IN ('open','in_progress')`,
		status, closedByUserID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return &apperr.Conflict{Resource: "stocktake_session", Message: "session is not open or in_progress"}
	}
	return nil
}

// ListSessions returns paginated sessions for a branch.
func (r *StocktakeRepo) ListSessions(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.StocktakeSession], error) {
	var countQ, dataQ string
	var args, dArgs []any
	if branchID != "" {
		countQ = `SELECT COUNT(*) FROM lms.stocktake_sessions WHERE branch_id=$1`
		dataQ = `SELECT id::text, branch_id::text, name, status,
		                started_at, closed_at, started_by::text, closed_by::text, notes, created_at
		         FROM lms.stocktake_sessions
		         WHERE branch_id=$1
		         ORDER BY started_at DESC LIMIT $2 OFFSET $3`
		args = []any{branchID}
		dArgs = []any{branchID, p.Limit(), p.Offset()}
	} else {
		countQ = `SELECT COUNT(*) FROM lms.stocktake_sessions`
		dataQ = `SELECT id::text, branch_id::text, name, status,
		                started_at, closed_at, started_by::text, closed_by::text, notes, created_at
		         FROM lms.stocktake_sessions
		         ORDER BY started_at DESC LIMIT $1 OFFSET $2`
		dArgs = []any{p.Limit(), p.Offset()}
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return model.PageResult[*model.StocktakeSession]{}, err
	}

	rows, err := r.pool.Query(ctx, dataQ, dArgs...)
	if err != nil {
		return model.PageResult[*model.StocktakeSession]{}, err
	}
	defer rows.Close()

	var items []*model.StocktakeSession
	for rows.Next() {
		s := &model.StocktakeSession{}
		if err := rows.Scan(
			&s.ID, &s.BranchID, &s.Name, &s.Status,
			&s.StartedAt, &s.ClosedAt, &s.StartedBy, &s.ClosedBy, &s.Notes, &s.CreatedAt,
		); err != nil {
			return model.PageResult[*model.StocktakeSession]{}, err
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return model.PageResult[*model.StocktakeSession]{}, err
	}
	return model.NewPageResult(items, total, p), nil
}

// RecordFinding inserts a finding. Returns apperr.Conflict if barcode already scanned in session.
func (r *StocktakeRepo) RecordFinding(ctx context.Context, f *model.StocktakeFinding) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO lms.stocktake_findings
		    (session_id, copy_id, scanned_barcode, finding_type, scanned_by, notes)
		VALUES ($1, $2::uuid, $3, $4, $5::uuid, $6)
		RETURNING id::text, scanned_at`,
		f.SessionID, f.CopyID, f.ScannedBarcode, f.FindingType, f.ScannedBy, f.Notes,
	).Scan(&f.ID, &f.ScannedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return &apperr.Conflict{Resource: "stocktake_finding", Message: "barcode already scanned in this session"}
		}
		return err
	}
	return nil
}

// GetFinding returns an existing finding for a (session, barcode) pair.
func (r *StocktakeRepo) GetFinding(ctx context.Context, sessionID, barcode string) (*model.StocktakeFinding, error) {
	f := &model.StocktakeFinding{}
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, session_id::text, copy_id::text,
		       scanned_barcode, finding_type, scanned_at, scanned_by::text, notes
		FROM lms.stocktake_findings
		WHERE session_id = $1 AND scanned_barcode = $2`,
		sessionID, barcode,
	).Scan(&f.ID, &f.SessionID, &f.CopyID,
		&f.ScannedBarcode, &f.FindingType, &f.ScannedAt, &f.ScannedBy, &f.Notes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "stocktake_finding", ID: barcode}
		}
		return nil, err
	}
	return f, nil
}

// ListFindings returns paginated findings for a session.
func (r *StocktakeRepo) ListFindings(ctx context.Context, sessionID string, p model.Pagination) (model.PageResult[*model.StocktakeFinding], error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lms.stocktake_findings WHERE session_id=$1`, sessionID).Scan(&total); err != nil {
		return model.PageResult[*model.StocktakeFinding]{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, session_id::text, copy_id::text,
		       scanned_barcode, finding_type, scanned_at, scanned_by::text, notes
		FROM lms.stocktake_findings
		WHERE session_id = $1
		ORDER BY scanned_at DESC
		LIMIT $2 OFFSET $3`,
		sessionID, p.Limit(), p.Offset(),
	)
	if err != nil {
		return model.PageResult[*model.StocktakeFinding]{}, err
	}
	defer rows.Close()

	var items []*model.StocktakeFinding
	for rows.Next() {
		f := &model.StocktakeFinding{}
		if err := rows.Scan(&f.ID, &f.SessionID, &f.CopyID,
			&f.ScannedBarcode, &f.FindingType, &f.ScannedAt, &f.ScannedBy, &f.Notes); err != nil {
			return model.PageResult[*model.StocktakeFinding]{}, err
		}
		items = append(items, f)
	}
	if err := rows.Err(); err != nil {
		return model.PageResult[*model.StocktakeFinding]{}, err
	}
	return model.NewPageResult(items, total, p), nil
}

// GetVariances detects three categories of discrepancy for a closed/in-progress session:
//
//	"missing"    – copy belongs to the branch but was not scanned in this session
//	"unexpected" – scanned barcode has no matching copy in the system
//	"misplaced"  – copy was scanned and found but its home branch differs from session branch
func (r *StocktakeRepo) GetVariances(ctx context.Context, sessionID, branchID string) ([]stocktake.VarianceItem, error) {
	var out []stocktake.VarianceItem

	// ── 1. Missing copies ──────────────────────────────────────────────────────
	// Copies in the branch (not checked_out / in_transit / withdrawn) that
	// do NOT appear as a 'found' or 'damaged' finding in this session.
	missingRows, err := r.pool.Query(ctx, `
		SELECT c.id::text, c.barcode, h.id::text, h.title, h.author, c.status_code
		FROM lms.copies c
		JOIN lms.holdings h ON h.id = c.holding_id
		WHERE c.branch_id = $1::uuid
		  AND c.status_code NOT IN ('checked_out','in_transit','withdrawn')
		  AND c.id NOT IN (
		      SELECT copy_id FROM lms.stocktake_findings
		      WHERE session_id = $2::uuid
		        AND copy_id IS NOT NULL
		        AND finding_type IN ('found','damaged')
		  )
		ORDER BY c.barcode`,
		branchID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("missing query: %w", err)
	}
	defer missingRows.Close()
	for missingRows.Next() {
		var v stocktake.VarianceItem
		v.Type = "missing"
		if err := missingRows.Scan(&v.CopyID, &v.Barcode, &v.HoldingID, &v.Title, &v.Author, &v.StatusCode); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := missingRows.Err(); err != nil {
		return nil, err
	}

	// ── 2. Unexpected barcodes ─────────────────────────────────────────────────
	// Scans where copy_id is NULL (barcode not in the system).
	unexpRows, err := r.pool.Query(ctx, `
		SELECT scanned_barcode, notes
		FROM lms.stocktake_findings
		WHERE session_id = $1::uuid AND copy_id IS NULL
		ORDER BY scanned_at`,
		sessionID)
	if err != nil {
		return nil, fmt.Errorf("unexpected query: %w", err)
	}
	defer unexpRows.Close()
	for unexpRows.Next() {
		var v stocktake.VarianceItem
		v.Type = "unexpected"
		if err := unexpRows.Scan(&v.Barcode, &v.Notes); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := unexpRows.Err(); err != nil {
		return nil, err
	}

	// ── 3. Misplaced copies ────────────────────────────────────────────────────
	// Scans that matched a copy, but the copy's branch_id differs from the session branch.
	misplacedRows, err := r.pool.Query(ctx, `
		SELECT c.id::text, c.barcode, h.id::text, h.title, h.author, c.status_code, c.branch_id::text
		FROM lms.stocktake_findings sf
		JOIN lms.copies c ON c.id = sf.copy_id
		JOIN lms.holdings h ON h.id = c.holding_id
		WHERE sf.session_id = $1::uuid
		  AND sf.copy_id IS NOT NULL
		  AND c.branch_id != $2::uuid
		ORDER BY c.barcode`,
		sessionID, branchID)
	if err != nil {
		return nil, fmt.Errorf("misplaced query: %w", err)
	}
	defer misplacedRows.Close()
	for misplacedRows.Next() {
		var v stocktake.VarianceItem
		v.Type = "misplaced"
		if err := misplacedRows.Scan(&v.CopyID, &v.Barcode, &v.HoldingID, &v.Title, &v.Author, &v.StatusCode, &v.HomeBranchID); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := misplacedRows.Err(); err != nil {
		return nil, err
	}

	if out == nil {
		out = []stocktake.VarianceItem{}
	}
	return out, nil
}

// HasActiveSession checks whether the branch already has an open or in_progress session.
func (r *StocktakeRepo) HasActiveSession(ctx context.Context, branchID string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM lms.stocktake_sessions
		WHERE branch_id=$1 AND status IN ('open','in_progress')`, branchID).Scan(&count)
	return count > 0, err
}
