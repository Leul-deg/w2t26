// Package db manages the PostgreSQL connection pool for the application.
// All database access goes through the pool returned by Connect; no raw
// connection strings are used outside this package.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool.Pool so callers import only this package.
type Pool = pgxpool.Pool

// Connect opens and validates a PostgreSQL connection pool using the provided
// DSN. It performs a ping to confirm the database is reachable before
// returning. Returns an error if the DSN is invalid, the database is
// unreachable, or the ping fails within the timeout.
func Connect(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: invalid DSN: %w", err)
	}

	// Conservative pool settings suitable for a single-machine deployment.
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: failed to create connection pool: %w", err)
	}

	// Confirm connectivity with a 10-second deadline.
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: database ping failed — is PostgreSQL running and reachable? %w", err)
	}

	return pool, nil
}
