// Package testdb provides helpers for integration tests that require a live
// PostgreSQL database. Tests that import this package require DATABASE_TEST_URL
// to be set in the environment (or a .env file in the backend directory).
//
// The test database is expected to be a separate database from the development
// database. Migrations are applied before the first test and rolled back after.
package testdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// Open returns a connection pool for the test database.
// The caller is responsible for calling pool.Close() when done.
// The function skips the test (t.Skip) if DATABASE_TEST_URL is not configured.
func Open(t *testing.T) *pgxpool.Pool {
	t.Helper()

	// Try to load .env from the backend directory.
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	dsn := os.Getenv("DATABASE_TEST_URL")
	if dsn == "" {
		// Fall back to constructing from individual vars (useful in CI).
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("DATABASE_TEST_URL not set — skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("testdb.Open: failed to create pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("testdb.Open: database ping failed: %v", err)
	}

	return pool
}

// ApplyMigrations runs all UP migrations against the test database.
// migrationsPath should be the absolute or relative path to the migrations/ directory.
func ApplyMigrations(t *testing.T, dsn, migrationsPath string) {
	t.Helper()

	m, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		t.Fatalf("testdb.ApplyMigrations: create migrator: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("testdb.ApplyMigrations: apply: %v", err)
	}
}

// Exec runs a SQL statement against the pool, failing the test on error.
func Exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql, args...)
	if err != nil {
		t.Fatalf("testdb.Exec: %v\nSQL: %s", err, sql)
	}
}

// QueryOne executes a query expected to return a single row and scans the
// result into dest. Fails the test if zero rows are returned.
func QueryOne(t *testing.T, pool *pgxpool.Pool, dest any, sql string, args ...any) {
	t.Helper()
	row := pool.QueryRow(context.Background(), sql, args...)
	if err := row.Scan(dest); err != nil {
		t.Fatalf("testdb.QueryOne: %v\nSQL: %s", err, sql)
	}
}

// AssertConstraintError fails the test unless the error message contains
// the expected PostgreSQL constraint name.
func AssertConstraintError(t *testing.T, err error, constraintName string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected constraint violation for %q but got nil error", constraintName)
	}
	msg := err.Error()
	if !contains(msg, constraintName) {
		t.Fatalf("expected constraint violation for %q but got: %v", constraintName, err)
	}
}

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error, context string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", context, err)
	}
}

// DSN returns the test database DSN from the environment.
func DSN(t *testing.T) string {
	t.Helper()
	_ = godotenv.Load("../../.env")
	dsn := os.Getenv("DATABASE_TEST_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("DATABASE_TEST_URL not set — skipping integration test")
	}
	return dsn
}

// MigrationsPath returns the resolved migrations directory path.
// Tries several candidate paths relative to the test binary location.
func MigrationsPath() string {
	candidates := []string{
		"../../migrations",
		"../../../migrations",
		"/home/leul/Documents/w2t26/repo/migrations",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			if p, err := absolutePath(c); err == nil {
				return p
			}
		}
	}
	return candidates[0]
}

func absolutePath(p string) (string, error) {
	if len(p) > 0 && p[0] == '/' {
		return p, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", wd, p), nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			indexString(s, substr) >= 0)
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
