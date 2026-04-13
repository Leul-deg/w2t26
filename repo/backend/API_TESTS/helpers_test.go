package apitests

// helpers_test.go provides small utilities shared across API test files.

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// contextBG is a shorthand for context.Background() used in cleanup closures.
func contextBG() context.Context {
	return context.Background()
}

// execIgnoreErr runs a SQL statement on the pool, silently ignoring errors.
// Use only in t.Cleanup callbacks for best-effort teardown.
func execIgnoreErr(pool *pgxpool.Pool, sql string, args ...any) {
	pool.Exec(context.Background(), sql, args...) //nolint
}

// opsUserCookie creates an operations_staff user scoped to mainBranchID and returns
// a logged-in session cookie. Used for copy write operations where admin's empty
// branch scope would cause branch-required DB inserts to fail.
func opsUserCookie(t *testing.T, app *completeTestApp) *http.Cookie {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, mainBranchID)
	return loginAs(t, app.testApp, username, "Password123!")
}
