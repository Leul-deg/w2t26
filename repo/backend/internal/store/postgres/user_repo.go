// Package postgres provides PostgreSQL implementations of all domain repositories.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/model"
)

// UserRepo implements users.Repository against the lms schema.
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo backed by the given connection pool.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// Create inserts a new user record. Returns apperr.Conflict on duplicate username or email.
func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO lms.users (username, email, password_hash, is_active)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id::text, created_at, updated_at`,
		u.Username, u.Email, u.PasswordHash, u.IsActive,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return &apperr.Conflict{Resource: "user", Message: "username or email already in use"}
		}
		return err
	}
	return nil
}

// GetByID returns the user with the given ID, or apperr.NotFound.
func (r *UserRepo) GetByID(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id::text, username, email, password_hash, is_active,
		        failed_attempts, locked_until, last_login_at, created_at, updated_at
		 FROM lms.users
		 WHERE id = $1`,
		id,
	).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsActive,
		&u.FailedAttempts, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "user", ID: id}
		}
		return nil, err
	}
	return u, nil
}

// GetByUsername returns the user with the given username, or apperr.NotFound.
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	u := &model.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id::text, username, email, password_hash, is_active,
		        failed_attempts, locked_until, last_login_at, created_at, updated_at
		 FROM lms.users
		 WHERE username = $1`,
		username,
	).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsActive,
		&u.FailedAttempts, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &apperr.NotFound{Resource: "user", ID: username}
		}
		return nil, err
	}
	return u, nil
}

// Update persists changes to an existing user record.
func (r *UserRepo) Update(ctx context.Context, u *model.User) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.users
		 SET username=$1, email=$2, is_active=$3, updated_at=NOW()
		 WHERE id=$4`,
		u.Username, u.Email, u.IsActive, u.ID,
	)
	return err
}

// IncrementFailedAttempts atomically increments failed_attempts and returns the new count.
func (r *UserRepo) IncrementFailedAttempts(ctx context.Context, id string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`UPDATE lms.users
		 SET failed_attempts = failed_attempts + 1, updated_at = NOW()
		 WHERE id = $1
		 RETURNING failed_attempts`,
		id,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ResetFailedAttempts sets failed_attempts to 0 and clears locked_until.
func (r *UserRepo) ResetFailedAttempts(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.users
		 SET failed_attempts = 0, locked_until = NULL, updated_at = NOW()
		 WHERE id = $1`,
		id,
	)
	return err
}

// SetLockedUntil sets the lockout expiry timestamp for an account.
func (r *UserRepo) SetLockedUntil(ctx context.Context, id string, until time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.users
		 SET locked_until = $1, updated_at = NOW()
		 WHERE id = $2`,
		until, id,
	)
	return err
}

// SetLastLogin updates last_login_at to the current timestamp.
func (r *UserRepo) SetLastLogin(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lms.users
		 SET last_login_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id,
	)
	return err
}

// List returns all users, optionally filtered by branch assignment.
func (r *UserRepo) List(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.User], error) {
	var (
		rows pgx.Rows
		err  error
	)

	if branchID == "" {
		rows, err = r.pool.Query(ctx,
			`SELECT id::text, username, email, password_hash, is_active,
			        failed_attempts, locked_until, last_login_at, created_at, updated_at
			 FROM lms.users
			 ORDER BY username
			 LIMIT $1 OFFSET $2`,
			p.Limit(), p.Offset(),
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT u.id::text, u.username, u.email, u.password_hash, u.is_active,
			        u.failed_attempts, u.locked_until, u.last_login_at, u.created_at, u.updated_at
			 FROM lms.users u
			 JOIN lms.user_branch_assignments uba ON uba.user_id = u.id
			 WHERE uba.branch_id = $1
			 ORDER BY u.username
			 LIMIT $2 OFFSET $3`,
			branchID, p.Limit(), p.Offset(),
		)
	}
	if err != nil {
		return model.PageResult[*model.User]{}, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsActive,
			&u.FailedAttempts, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return model.PageResult[*model.User]{}, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return model.PageResult[*model.User]{}, err
	}

	var total int
	if branchID == "" {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM lms.users`).Scan(&total)
	} else {
		err = r.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM lms.users u
			 JOIN lms.user_branch_assignments uba ON uba.user_id = u.id
			 WHERE uba.branch_id = $1`, branchID).Scan(&total)
	}
	if err != nil {
		return model.PageResult[*model.User]{}, err
	}

	return model.NewPageResult(users, total, p), nil
}

// GetRoles returns all roles assigned to the given user.
func (r *UserRepo) GetRoles(ctx context.Context, userID string) ([]*model.Role, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT r.id::text, r.name, r.description, r.created_at
		 FROM lms.roles r
		 JOIN lms.user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*model.Role
	for rows.Next() {
		role := &model.Role{}
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

// GetPermissions returns the distinct permission names for the given user via their roles.
func (r *UserRepo) GetPermissions(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT p.name
		 FROM lms.permissions p
		 JOIN lms.role_permissions rp ON rp.permission_id = p.id
		 JOIN lms.user_roles ur ON ur.role_id = rp.role_id
		 WHERE ur.user_id = $1
		 ORDER BY p.name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		perms = append(perms, name)
	}
	return perms, rows.Err()
}

// AssignRole assigns a role to a user (idempotent via ON CONFLICT DO NOTHING).
func (r *UserRepo) AssignRole(ctx context.Context, userID, roleID, assignedByID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO lms.user_roles (user_id, role_id, assigned_by)
		 VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		userID, roleID, assignedByID,
	)
	return err
}

// RevokeRole removes a role assignment from a user.
func (r *UserRepo) RevokeRole(ctx context.Context, userID, roleID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lms.user_roles WHERE user_id = $1 AND role_id = $2`,
		userID, roleID,
	)
	return err
}

// GetBranches returns the branch IDs assigned to the given user.
func (r *UserRepo) GetBranches(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT branch_id::text FROM lms.user_branch_assignments WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branchIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		branchIDs = append(branchIDs, id)
	}
	return branchIDs, rows.Err()
}

// AssignBranch assigns a user to a branch (idempotent via ON CONFLICT DO NOTHING).
func (r *UserRepo) AssignBranch(ctx context.Context, userID, branchID, assignedByID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO lms.user_branch_assignments (user_id, branch_id, assigned_by)
		 VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		userID, branchID, assignedByID,
	)
	return err
}

// RevokeBranch removes a branch assignment from a user.
func (r *UserRepo) RevokeBranch(ctx context.Context, userID, branchID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lms.user_branch_assignments WHERE user_id = $1 AND branch_id = $2`,
		userID, branchID,
	)
	return err
}

// isUniqueViolation checks whether the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return containsStr(err.Error(), "unique") || containsStr(err.Error(), "23505")
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexStr(s, substr) >= 0)
}

func indexStr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
