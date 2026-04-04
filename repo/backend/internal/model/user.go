// Package model contains pure domain types that mirror the database schema.
// These structs have no dependencies on database drivers or HTTP frameworks.
// Nil pointer fields represent nullable columns.
package model

import "time"

// User represents a staff account in the system.
type User struct {
	ID             string     `json:"id"`
	Username       string     `json:"username"`
	Email          string     `json:"email"`
	PasswordHash   string     `json:"-"` // never serialised to JSON
	IsActive       bool       `json:"is_active"`
	FailedAttempts int        `json:"failed_attempts"`
	LockedUntil    *time.Time `json:"locked_until,omitempty"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// IsLocked returns true if the user account is currently locked out.
func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && u.LockedUntil.After(now)
}

// Role represents a named set of permissions (administrator, operations_staff, content_moderator).
type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Permission represents a named capability token (e.g. "readers:write").
type Permission struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserRole is the assignment of a role to a user.
type UserRole struct {
	UserID     string     `json:"user_id"`
	RoleID     string     `json:"role_id"`
	AssignedAt time.Time  `json:"assigned_at"`
	AssignedBy *string    `json:"assigned_by,omitempty"`
}

// Session represents an active server-side session.
type Session struct {
	ID            string     `json:"id"`
	TokenHash     string     `json:"-"` // never sent to client
	UserID        string     `json:"user_id"`
	WorkstationID *string    `json:"workstation_id,omitempty"`
	IPAddress     *string    `json:"ip_address,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastActiveAt  time.Time  `json:"last_active_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	IsValid       bool       `json:"is_valid"`
	StepUpAt      *time.Time `json:"-"` // never serialised; used to gate reveal-sensitive
}

// HasRecentStepUp returns true if a step-up was completed within the given window.
func (s *Session) HasRecentStepUp(windowMinutes int) bool {
	if s == nil || s.StepUpAt == nil {
		return false
	}
	return time.Since(*s.StepUpAt) <= time.Duration(windowMinutes)*time.Minute
}

// CaptchaChallenge is issued to a client after repeated login failures.
type CaptchaChallenge struct {
	ID           string    `json:"id"`
	ChallengeKey string    `json:"challenge_key"` // sent to client
	AnswerHash   string    `json:"-"`              // never sent to client
	Username     *string   `json:"-"`
	IPAddress    *string   `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Used         bool      `json:"used"`
}

// UserWithRoles bundles a user with their resolved role names and permissions.
// Used for auth context after a successful login.
type UserWithRoles struct {
	User        *User    `json:"user"`
	Roles       []string `json:"roles"`        // role names
	Permissions []string `json:"permissions"`  // permission names (e.g. "readers:write")
}

// HasPermission returns true if the user holds the named permission.
func (u *UserWithRoles) HasPermission(p string) bool {
	for _, perm := range u.Permissions {
		if perm == p {
			return true
		}
	}
	return false
}
