package main

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// runMigrationsInternal applies all pending UP migrations from the given
// directory. The directory must contain files named NNN_description.up.sql
// and NNN_description.down.sql (golang-migrate convention).
//
// If no migrations are pending the function returns nil. Any other error
// is propagated so the caller can decide whether to abort startup.
func runMigrationsInternal(dsn, migrationsPath string) error {
	sourceURL := "file://" + migrationsPath

	m, err := migrate.New(sourceURL, dsn)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			// No pending migrations — this is normal after the first run.
			return nil
		}
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
