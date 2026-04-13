package apitests

// helpers_test.go provides small utilities shared across API test files.

import (
	"context"

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
