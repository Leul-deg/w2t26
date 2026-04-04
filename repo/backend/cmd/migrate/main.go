// Command migrate is a standalone CLI tool for managing database schema
// migrations. It supports up, down, and version subcommands.
//
// Usage:
//
//	go run ./cmd/migrate up       # apply all pending migrations
//	go run ./cmd/migrate down 1   # roll back the last N migrations
//	go run ./cmd/migrate version  # print the current migration version
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "../../migrations"
	}

	if len(args) == 0 {
		return fmt.Errorf("usage: migrate <up|down|version> [steps]")
	}

	sourceURL := "file://" + migrationsPath
	m, err := migrate.New(sourceURL, dsn)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	switch args[0] {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate up: %w", err)
		}
		v, _, _ := m.Version()
		slog.Info("migrations applied", "current_version", v)

	case "down":
		steps := 1
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 {
				return fmt.Errorf("down requires a positive integer step count, got %q", args[1])
			}
			steps = n
		}
		if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate down %d: %w", steps, err)
		}
		v, _, _ := m.Version()
		slog.Info("migrations rolled back", "steps", steps, "current_version", v)

	case "version":
		v, dirty, err := m.Version()
		if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
			return fmt.Errorf("get version: %w", err)
		}
		slog.Info("migration version", "version", v, "dirty", dirty)

	default:
		return fmt.Errorf("unknown subcommand %q — valid subcommands: up, down, version", args[0])
	}

	return nil
}
