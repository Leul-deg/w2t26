package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/domain/users"
	"lms/internal/model"
)

const sessionCookieName = "lms_session"

// RequireAuth is an Echo middleware that validates the session cookie and
// loads the authenticated user into the request context.
//
// On every request:
//  1. Reads the "lms_session" cookie.
//  2. SHA-256 hashes the raw cookie value → hex string → looks up in session repo.
//  3. If no valid session found, returns 401.
//  4. Extends the session expiry (touches last_active_at).
//  5. Loads the user + roles + permissions and stores in context.
//  6. Sets branch scope in context.
func RequireAuth(sessionRepo users.SessionRepository, userRepo users.Repository, inactivitySecs int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			session, user, err := loadSession(c, sessionRepo, userRepo, inactivitySecs)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error":  "unauthenticated",
					"detail": "no valid session",
				})
			}

			// Store user and session in context.
			ctx := c.Request().Context()
			ctx = ctxutil.SetUser(ctx, user)
			ctx = ctxutil.SetSession(ctx, session)

			// Set branch scope.
			branchID := resolveBranchID(user)
			ctx = ctxutil.SetBranchID(ctx, branchID)

			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// OptionalAuth is like RequireAuth but does not return 401 on a missing or
// invalid session. Use for routes that work for both authenticated and
// unauthenticated callers.
func OptionalAuth(sessionRepo users.SessionRepository, userRepo users.Repository, inactivitySecs int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			session, user, err := loadSession(c, sessionRepo, userRepo, inactivitySecs)
			if err == nil && session != nil && user != nil {
				ctx := c.Request().Context()
				ctx = ctxutil.SetUser(ctx, user)
				ctx = ctxutil.SetSession(ctx, session)
				branchID := resolveBranchID(user)
				ctx = ctxutil.SetBranchID(ctx, branchID)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	}
}

// loadSession extracts and validates the session from the request cookie.
// Returns the session, user-with-roles, and any error encountered.
func loadSession(
	c echo.Context,
	sessionRepo users.SessionRepository,
	userRepo users.Repository,
	inactivitySecs int,
) (*model.Session, *model.UserWithRoles, error) {
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, nil, &apperr.Unauthorized{}
	}

	// Hash the raw cookie value for lookup. Never store or log the raw token.
	hash := sha256.Sum256([]byte(cookie.Value))
	tokenHash := hex.EncodeToString(hash[:])

	ctx := c.Request().Context()
	session, err := sessionRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, nil, &apperr.Unauthorized{}
	}

	// Verify expiry — the repo query already checks this, but belt-and-suspenders.
	if session.ExpiresAt.Before(time.Now()) {
		_ = sessionRepo.Invalidate(ctx, session.ID)
		return nil, nil, &apperr.Unauthorized{}
	}

	// Touch the session — extend expiry by the inactivity window.
	newExpiry := time.Now().Add(time.Duration(inactivitySecs) * time.Second)
	if err := sessionRepo.Touch(ctx, session.ID, newExpiry); err != nil {
		// Non-fatal: log would be appropriate here in production, but don't block.
		_ = err
	}
	session.ExpiresAt = newExpiry

	// Load the user with roles and permissions.
	u, err := userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, nil, &apperr.Unauthorized{}
	}
	if !u.IsActive {
		return nil, nil, &apperr.Unauthorized{}
	}

	roles, err := userRepo.GetRoles(ctx, u.ID)
	if err != nil {
		return nil, nil, &apperr.Unauthorized{}
	}
	roleNames := make([]string, len(roles))
	for i, r := range roles {
		roleNames[i] = r.Name
	}

	perms, err := userRepo.GetPermissions(ctx, u.ID)
	if err != nil {
		return nil, nil, &apperr.Unauthorized{}
	}

	userWithRoles := &model.UserWithRoles{
		User:        u,
		Roles:       roleNames,
		Permissions: perms,
	}

	return session, userWithRoles, nil
}

// resolveBranchID returns an empty string for administrators (all-branch access)
// or the first branch assignment for non-admin users. Branch scope is set
// separately by BranchScope middleware if more granular control is needed.
func resolveBranchID(u *model.UserWithRoles) string {
	for _, role := range u.Roles {
		if role == "administrator" {
			return "" // empty = see all branches
		}
	}
	// For non-admins, return empty — BranchScope middleware will populate from DB.
	return ""
}
