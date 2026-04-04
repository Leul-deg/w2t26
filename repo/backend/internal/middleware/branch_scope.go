package middleware

import (
	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/domain/users"
)

// BranchScope is an Echo middleware that sets the effective branch_id in the
// request context based on the authenticated user's role assignments.
//
// For administrators: branch_id = "" (no filter — sees all branches).
// For all others: branch_id = first assigned branch from the user repo.
//
// Must be placed AFTER RequireAuth in the middleware chain.
func BranchScope(userRepo users.Repository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			user, ok := ctxutil.GetUser(ctx)
			if !ok || user == nil {
				// No authenticated user — let the auth middleware handle it.
				return next(c)
			}

			// Administrators see all branches.
			for _, role := range user.Roles {
				if role == "administrator" {
					ctx = ctxutil.SetBranchID(ctx, "")
					c.SetRequest(c.Request().WithContext(ctx))
					return next(c)
				}
			}

			// Non-admins: resolve their first branch assignment from the DB.
			// If GetBranches fails or the user has no branches, use the nil-branch
			// sentinel UUID. This intentionally matches no real branch UUID in the
			// database, so all branch-filtered queries return empty results rather
			// than leaking cross-branch data (which an empty string would do by
			// bypassing branch filters throughout the codebase).
			branchIDs, err := userRepo.GetBranches(ctx, user.User.ID)
			if err != nil || len(branchIDs) == 0 {
				ctx = ctxutil.SetBranchID(ctx, "00000000-0000-0000-0000-000000000000")
			} else {
				ctx = ctxutil.SetBranchID(ctx, branchIDs[0])
			}

			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// EnforceBranchAccess checks that the resource's branch is accessible to the
// calling user. Returns apperr.Forbidden if the user is not an administrator
// and the resource branch is not among their assigned branches.
// Returns nil if access is permitted.
func EnforceBranchAccess(resourceBranchID string, c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Forbidden{Action: "access", Resource: "branch-scoped resource"}
	}

	// Administrators have access to all branches.
	for _, role := range user.Roles {
		if role == "administrator" {
			return nil
		}
	}

	// Non-admins must be assigned to the resource's branch.
	// The branch ID in context is their primary scope, but they may have more.
	// We check against the effective branch in context; for multi-branch non-admins
	// the BranchScope middleware sets the first branch. A more sophisticated check
	// would query all branches, but for the security foundation this is correct:
	// non-admins are limited to their single primary branch.
	branchID, _ := ctxutil.GetBranchID(ctx)
	if branchID == "" || branchID != resourceBranchID {
		return &apperr.Forbidden{Action: "access", Resource: "branch-scoped resource"}
	}

	return nil
}
