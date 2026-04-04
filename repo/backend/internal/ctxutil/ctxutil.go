// Package ctxutil provides typed context key helpers for the LMS application.
// These helpers avoid raw string keys in context and provide type-safe access
// to auth state set by the session middleware.
package ctxutil

import (
	"context"
	"net"

	"github.com/labstack/echo/v4"

	"lms/internal/model"
)

// contextKey is an unexported type for context keys to avoid collisions with
// other packages that may use plain strings as context keys.
type contextKey string

const (
	keyUser    contextKey = "auth_user"
	keySession contextKey = "auth_session"
	keyBranch  contextKey = "branch_id"
)

// SetUser stores a UserWithRoles into the context. Called by RequireAuth middleware.
func SetUser(ctx context.Context, u *model.UserWithRoles) context.Context {
	return context.WithValue(ctx, keyUser, u)
}

// GetUser retrieves the authenticated user from the context.
// Returns (nil, false) if no user is present (e.g. unauthenticated route).
func GetUser(ctx context.Context) (*model.UserWithRoles, bool) {
	u, ok := ctx.Value(keyUser).(*model.UserWithRoles)
	return u, ok && u != nil
}

// MustGetUser retrieves the authenticated user from the context.
// Panics if no user is present — use only in handlers guarded by RequireAuth.
func MustGetUser(ctx context.Context) *model.UserWithRoles {
	u, ok := GetUser(ctx)
	if !ok {
		panic("ctxutil: MustGetUser called outside RequireAuth middleware")
	}
	return u
}

// SetSession stores a Session into the context. Called by RequireAuth middleware.
func SetSession(ctx context.Context, s *model.Session) context.Context {
	return context.WithValue(ctx, keySession, s)
}

// GetSession retrieves the active session from the context.
// Returns (nil, false) if no session is present.
func GetSession(ctx context.Context) (*model.Session, bool) {
	s, ok := ctx.Value(keySession).(*model.Session)
	return s, ok && s != nil
}

// SetBranchID stores the effective branch ID scope into the context.
// An empty string means "all branches" (administrator scope).
func SetBranchID(ctx context.Context, branchID string) context.Context {
	return context.WithValue(ctx, keyBranch, branchID)
}

// GetBranchID retrieves the effective branch ID from the context.
// Returns ("", false) if no branch ID has been set.
func GetBranchID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(keyBranch).(string)
	return v, ok
}

// GetWorkstationID reads the X-Workstation-ID header from the request.
// Falls back to the remote IP address if the header is absent.
func GetWorkstationID(c echo.Context) string {
	wsID := c.Request().Header.Get("X-Workstation-ID")
	if wsID != "" {
		return wsID
	}
	return GetIPAddress(c)
}

// GetIPAddress returns the client IP address using Echo's RealIP helper.
// Echo's RealIP respects X-Real-IP and X-Forwarded-For (when trusted proxy
// is configured) but falls back to RemoteAddr, which is reliable on a
// local network without a trusted reverse proxy.
func GetIPAddress(c echo.Context) string {
	ip := c.RealIP()
	if ip == "" {
		// Strip port from RemoteAddr as a last resort.
		host, _, err := net.SplitHostPort(c.Request().RemoteAddr)
		if err == nil {
			return host
		}
		return c.Request().RemoteAddr
	}
	return ip
}
