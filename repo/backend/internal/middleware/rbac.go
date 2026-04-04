package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"lms/internal/ctxutil"
)

// RequirePermission returns an Echo middleware that checks whether the
// authenticated user holds the named permission. Must be used AFTER RequireAuth.
// Returns 403 if the user lacks the permission.
func RequirePermission(perm string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := ctxutil.GetUser(c.Request().Context())
			if !ok || user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "unauthenticated",
				})
			}
			if !user.HasPermission(perm) {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "forbidden",
				})
			}
			return next(c)
		}
	}
}

// RequireRole returns an Echo middleware that checks whether the authenticated
// user holds the named role. Must be used AFTER RequireAuth.
// Returns 403 if the user lacks the role.
func RequireRole(role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := ctxutil.GetUser(c.Request().Context())
			if !ok || user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "unauthenticated",
				})
			}
			for _, r := range user.Roles {
				if r == role {
					return next(c)
				}
			}
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "forbidden",
			})
		}
	}
}
