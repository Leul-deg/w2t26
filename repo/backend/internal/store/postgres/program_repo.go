package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/programs"
	"lms/internal/model"
)

// ProgramRepo implements programs.Repository against the lms schema.
type ProgramRepo struct {
	pool *pgxpool.Pool
}

// NewProgramRepo creates a new ProgramRepo.
func NewProgramRepo(pool *pgxpool.Pool) *ProgramRepo {
	return &ProgramRepo{pool: pool}
}

const programSelectCols = `
	id::text, branch_id::text, title, description, category,
	venue_type, venue_name, capacity,
	enrollment_opens_at, enrollment_closes_at,
	starts_at, ends_at, status, enrollment_channel,
	created_by::text, created_at, updated_at`

func scanProgram(row pgx.Row) (*model.Program, error) {
	p := &model.Program{}
	err := row.Scan(
		&p.ID, &p.BranchID, &p.Title, &p.Description, &p.Category,
		&p.VenueType, &p.VenueName, &p.Capacity,
		&p.EnrollmentOpensAt, &p.EnrollmentClosesAt,
		&p.StartsAt, &p.EndsAt, &p.Status, &p.EnrollmentChannel,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

// Create inserts a new program.
func (r *ProgramRepo) Create(ctx context.Context, p *model.Program) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO lms.programs
		    (branch_id, title, description, category, venue_type, venue_name, capacity,
		     enrollment_opens_at, enrollment_closes_at,
		     starts_at, ends_at, status, enrollment_channel, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING `+programSelectCols,
		p.BranchID, p.Title, p.Description, p.Category, p.VenueType, p.VenueName, p.Capacity,
		p.EnrollmentOpensAt, p.EnrollmentClosesAt,
		p.StartsAt, p.EndsAt, p.Status, p.EnrollmentChannel, p.CreatedBy,
	).Scan(
		&p.ID, &p.BranchID, &p.Title, &p.Description, &p.Category,
		&p.VenueType, &p.VenueName, &p.Capacity,
		&p.EnrollmentOpensAt, &p.EnrollmentClosesAt,
		&p.StartsAt, &p.EndsAt, &p.Status, &p.EnrollmentChannel,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
}

// GetByID returns the program scoped to branchID.
func (r *ProgramRepo) GetByID(ctx context.Context, id, branchID string) (*model.Program, error) {
	q := `SELECT ` + programSelectCols + ` FROM lms.programs WHERE id = $1`
	args := []any{id}
	if branchID != "" {
		q += " AND branch_id = $2"
		args = append(args, branchID)
	}
	p, err := scanProgram(r.pool.QueryRow(ctx, q, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "program", ID: id}
		}
		return nil, err
	}
	return p, nil
}

// Update persists bibliographic changes to a program.
func (r *ProgramRepo) Update(ctx context.Context, p *model.Program) error {
	return r.pool.QueryRow(ctx, `
		UPDATE lms.programs
		SET title=$2, description=$3, category=$4, venue_type=$5, venue_name=$6,
		    capacity=$7, enrollment_opens_at=$8, enrollment_closes_at=$9,
		    starts_at=$10, ends_at=$11, enrollment_channel=$12, updated_at=NOW()
		WHERE id=$1
		RETURNING `+programSelectCols,
		p.ID, p.Title, p.Description, p.Category, p.VenueType, p.VenueName, p.Capacity,
		p.EnrollmentOpensAt, p.EnrollmentClosesAt,
		p.StartsAt, p.EndsAt, p.EnrollmentChannel,
	).Scan(
		&p.ID, &p.BranchID, &p.Title, &p.Description, &p.Category,
		&p.VenueType, &p.VenueName, &p.Capacity,
		&p.EnrollmentOpensAt, &p.EnrollmentClosesAt,
		&p.StartsAt, &p.EndsAt, &p.Status, &p.EnrollmentChannel,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
	)
}

// UpdateStatus changes only the program status, scoped to the caller's branch.
func (r *ProgramRepo) UpdateStatus(ctx context.Context, id, branchID, status string) error {
	var rowsAffected int64
	if branchID != "" {
		tag, err := r.pool.Exec(ctx,
			`UPDATE lms.programs SET status=$3, updated_at=NOW() WHERE id=$1 AND branch_id=$2`,
			id, branchID, status)
		if err != nil {
			return err
		}
		rowsAffected = tag.RowsAffected()
	} else {
		tag, err := r.pool.Exec(ctx,
			`UPDATE lms.programs SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
		if err != nil {
			return err
		}
		rowsAffected = tag.RowsAffected()
	}
	if rowsAffected == 0 {
		return &apperr.NotFound{Resource: "program", ID: id}
	}
	return nil
}

// List returns a paginated, filtered list of programs.
// An empty branchID returns programs across all branches (administrator scope).
func (r *ProgramRepo) List(ctx context.Context, branchID string, f programs.ProgramFilter, p model.Pagination) (model.PageResult[*model.Program], error) {
	var where []string
	var args []any
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
	if f.Category != nil {
		where = append(where, "category = $"+itoa(idx))
		args = append(args, *f.Category)
		idx++
	}
	if f.Search != nil {
		where = append(where, "title ILIKE $"+itoa(idx))
		args = append(args, "%"+*f.Search+"%")
		idx++
	}
	var whereClause string
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM lms.programs "+whereClause, args...,
	).Scan(&total); err != nil {
		return model.PageResult[*model.Program]{}, err
	}

	args = append(args, p.Limit(), p.Offset())
	rows, err := r.pool.Query(ctx,
		`SELECT `+programSelectCols+` FROM lms.programs `+whereClause+
			` ORDER BY starts_at ASC LIMIT $`+itoa(idx)+` OFFSET $`+itoa(idx+1),
		args...)
	if err != nil {
		return model.PageResult[*model.Program]{}, err
	}
	defer rows.Close()

	var items []*model.Program
	for rows.Next() {
		prog, err := scanProgram(rows)
		if err != nil {
			return model.PageResult[*model.Program]{}, err
		}
		items = append(items, prog)
	}
	if items == nil {
		items = []*model.Program{}
	}
	return model.NewPageResult(items, total, p), nil
}

// ── Prerequisites ─────────────────────────────────────────────────────────────

// GetPrerequisites returns all prerequisites for a program.
func (r *ProgramRepo) GetPrerequisites(ctx context.Context, programID string) ([]*model.ProgramPrerequisite, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, program_id::text, required_program_id::text, description, created_at
		FROM lms.program_prerequisites
		WHERE program_id = $1
		ORDER BY created_at`, programID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prereqs []*model.ProgramPrerequisite
	for rows.Next() {
		pr := &model.ProgramPrerequisite{}
		if err := rows.Scan(&pr.ID, &pr.ProgramID, &pr.RequiredProgramID, &pr.Description, &pr.CreatedAt); err != nil {
			return nil, err
		}
		prereqs = append(prereqs, pr)
	}
	if prereqs == nil {
		prereqs = []*model.ProgramPrerequisite{}
	}
	return prereqs, nil
}

// AddPrerequisite inserts a prerequisite relationship.
func (r *ProgramRepo) AddPrerequisite(ctx context.Context, pr *model.ProgramPrerequisite) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO lms.program_prerequisites (program_id, required_program_id, description)
		VALUES ($1, $2, $3)
		RETURNING id::text, created_at`,
		pr.ProgramID, pr.RequiredProgramID, pr.Description,
	).Scan(&pr.ID, &pr.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return &apperr.Conflict{Resource: "prerequisite", Message: "this prerequisite already exists"}
		}
		return err
	}
	return nil
}

// RemovePrerequisite deletes a prerequisite by the pair of program IDs.
func (r *ProgramRepo) RemovePrerequisite(ctx context.Context, programID, requiredProgramID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM lms.program_prerequisites
		WHERE program_id = $1 AND required_program_id = $2`, programID, requiredProgramID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return &apperr.NotFound{Resource: "prerequisite", ID: requiredProgramID}
	}
	return nil
}

// ── Enrollment rules ──────────────────────────────────────────────────────────

// GetEnrollmentRules returns all enrollment rules for a program.
func (r *ProgramRepo) GetEnrollmentRules(ctx context.Context, programID string) ([]*model.EnrollmentRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, program_id::text, rule_type, match_field, match_value, reason, created_at
		FROM lms.enrollment_rules
		WHERE program_id = $1
		ORDER BY rule_type, created_at`, programID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*model.EnrollmentRule
	for rows.Next() {
		rule := &model.EnrollmentRule{}
		if err := rows.Scan(&rule.ID, &rule.ProgramID, &rule.RuleType, &rule.MatchField, &rule.MatchValue, &rule.Reason, &rule.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	if rules == nil {
		rules = []*model.EnrollmentRule{}
	}
	return rules, nil
}

// AddEnrollmentRule inserts a new whitelist or blacklist rule.
func (r *ProgramRepo) AddEnrollmentRule(ctx context.Context, rule *model.EnrollmentRule) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO lms.enrollment_rules (program_id, rule_type, match_field, match_value, reason)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text, created_at`,
		rule.ProgramID, rule.RuleType, rule.MatchField, rule.MatchValue, rule.Reason,
	).Scan(&rule.ID, &rule.CreatedAt)
}

// RemoveEnrollmentRule deletes an enrollment rule by ID, scoped to programID.
// The AND program_id = $2 predicate prevents cross-program IDOR: a caller who
// knows a rule UUID from a different program cannot delete it by guessing.
func (r *ProgramRepo) RemoveEnrollmentRule(ctx context.Context, programID, ruleID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM lms.enrollment_rules WHERE id = $1 AND program_id = $2`, ruleID, programID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return &apperr.NotFound{Resource: "enrollment_rule", ID: ruleID}
	}
	return nil
}

// Ensure compile-time interface satisfaction.
var _ programs.Repository = (*ProgramRepo)(nil)

// itoa converts an int to its string representation for SQL placeholder building.
func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	result := make([]byte, 0, 4)
	for n > 0 {
		result = append([]byte{digits[n%10]}, result...)
		n /= 10
	}
	return string(result)
}
