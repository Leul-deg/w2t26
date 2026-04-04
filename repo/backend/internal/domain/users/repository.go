// Package users manages staff user accounts, roles, and permission resolution.
package users

import (
	"context"
	"time"

	"lms/internal/model"
)

// Repository defines the data access contract for user accounts.
// Implementations must enforce branch-scoped access where noted.
type Repository interface {
	// Create inserts a new user. Returns apperr.Conflict if username or email already exists.
	Create(ctx context.Context, u *model.User) error

	// GetByID returns the user with the given ID.
	// Returns apperr.NotFound if not found.
	GetByID(ctx context.Context, id string) (*model.User, error)

	// GetByUsername returns the user with the given username.
	// Returns apperr.NotFound if not found.
	GetByUsername(ctx context.Context, username string) (*model.User, error)

	// Update persists changes to an existing user record.
	Update(ctx context.Context, u *model.User) error

	// IncrementFailedAttempts atomically increments failed_attempts and returns the new count.
	IncrementFailedAttempts(ctx context.Context, id string) (int, error)

	// ResetFailedAttempts sets failed_attempts to 0 and clears locked_until.
	ResetFailedAttempts(ctx context.Context, id string) error

	// SetLockedUntil sets the lockout expiry for an account.
	SetLockedUntil(ctx context.Context, id string, until time.Time) error

	// SetLastLogin updates last_login_at to now.
	SetLastLogin(ctx context.Context, id string) error

	// List returns all users, optionally filtered by branch assignment.
	// branchID may be empty for administrator callers (no branch filter applied).
	List(ctx context.Context, branchID string, p model.Pagination) (model.PageResult[*model.User], error)

	// GetRoles returns all roles assigned to the given user.
	GetRoles(ctx context.Context, userID string) ([]*model.Role, error)

	// GetPermissions returns the distinct permission names for the given user.
	GetPermissions(ctx context.Context, userID string) ([]string, error)

	// AssignRole assigns a role to a user. Idempotent.
	AssignRole(ctx context.Context, userID, roleID, assignedByID string) error

	// RevokeRole removes a role from a user.
	RevokeRole(ctx context.Context, userID, roleID string) error

	// GetBranches returns the branch IDs assigned to the given user.
	GetBranches(ctx context.Context, userID string) ([]string, error)

	// AssignBranch assigns a user to a branch. Idempotent.
	AssignBranch(ctx context.Context, userID, branchID, assignedByID string) error

	// RevokeBranch removes a branch assignment from a user.
	RevokeBranch(ctx context.Context, userID, branchID string) error
}

// SessionRepository manages server-side sessions.
type SessionRepository interface {
	// Create stores a new session. tokenHash is hex(SHA-256(rawToken)).
	Create(ctx context.Context, s *model.Session) error

	// GetByTokenHash returns the session matching the token hash, if valid and not expired.
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error)

	// Touch updates last_active_at and extends expires_at by the inactivity window.
	Touch(ctx context.Context, id string, newExpiresAt time.Time) error

	// Invalidate marks the session as is_valid = false (logout or expiry).
	Invalidate(ctx context.Context, id string) error

	// InvalidateAll invalidates all active sessions for the given user.
	InvalidateAll(ctx context.Context, userID string) error

	// SetStepUp records the current time as the step-up timestamp on the session.
	// Called after a successful POST /auth/stepup to enable reveal-sensitive access.
	SetStepUp(ctx context.Context, sessionID string) error

	// PruneExpired deletes sessions that have been expired for more than retainFor duration.
	PruneExpired(ctx context.Context) (int64, error)
}

// CaptchaRepository manages local CAPTCHA challenges.
type CaptchaRepository interface {
	// Create stores a new CAPTCHA challenge.
	Create(ctx context.Context, c *model.CaptchaChallenge) error

	// GetByKey returns the challenge for the given key, if not used and not expired.
	GetByKey(ctx context.Context, key string) (*model.CaptchaChallenge, error)

	// MarkUsed marks the challenge as used. Must be called even on wrong answers.
	MarkUsed(ctx context.Context, id string) error

	// PruneExpired deletes expired and used challenges.
	PruneExpired(ctx context.Context) (int64, error)
}
