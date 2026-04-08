// Package config loads and validates application configuration from environment
// variables. A .env file in the working directory is loaded automatically if
// present; values already set in the environment take precedence.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration. Sensitive fields are never logged.
type Config struct {
	Server   ServerConfig
	DB       DBConfig
	Session  SessionConfig
	Crypto   CryptoConfig
	Migrate  MigrateConfig
	LogLevel string
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string
	Port int
}

// DBConfig holds PostgreSQL connection settings.
// DSN is treated as sensitive and must not be logged.
type DBConfig struct {
	DSN string // e.g. postgres://user:pass@host:5432/dbname?sslmode=disable
}

// SessionConfig holds session management settings.
type SessionConfig struct {
	// InactivityTimeoutSeconds is the number of seconds of inactivity after
	// which a session is considered expired. Default: 1800 (30 minutes).
	InactivityTimeoutSeconds int
}

// CryptoConfig holds field-level encryption settings.
type CryptoConfig struct {
	// KeyFile is the path to the 32-byte AES-256 key file used for
	// encrypting sensitive fields at rest.
	KeyFile string
}

// MigrateConfig holds migration runner settings.
type MigrateConfig struct {
	// Path is the filesystem path to the directory containing SQL migration files.
	Path string
}

// Load reads configuration from environment variables (and an optional .env
// file). Returns an error describing every missing or invalid required field
// so the caller gets one consolidated failure message.
func Load() (*Config, error) {
	// Load .env if present; silently continue if not found.
	_ = godotenv.Load()

	var errs []string

	cfg := &Config{}

	// --- Server ---
	cfg.Server.Host = envOr("SERVER_HOST", "0.0.0.0")
	port, err := envInt("SERVER_PORT", 8080)
	if err != nil {
		errs = append(errs, "SERVER_PORT: "+err.Error())
	}
	cfg.Server.Port = port

	// --- Database ---
	cfg.DB.DSN = os.Getenv("DATABASE_URL")
	if cfg.DB.DSN == "" {
		errs = append(errs, "DATABASE_URL: required, must be a valid PostgreSQL DSN")
	}

	// --- Session ---
	inactivity, err := envInt("SESSION_INACTIVITY_SECONDS", 1800)
	if err != nil {
		errs = append(errs, "SESSION_INACTIVITY_SECONDS: "+err.Error())
	}
	cfg.Session.InactivityTimeoutSeconds = inactivity

	// --- Crypto ---
	cfg.Crypto.KeyFile = os.Getenv("CRYPTO_KEY_FILE")
	if cfg.Crypto.KeyFile == "" {
		errs = append(errs, "CRYPTO_KEY_FILE: required, path to 32-byte AES-256 key file")
	}

	// --- Migrations ---
	cfg.Migrate.Path = envOr("MIGRATIONS_PATH", "../../migrations")

	// --- Logging ---
	cfg.LogLevel = envOr("LOG_LEVEL", "info")

	if len(errs) > 0 {
		return nil, fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return cfg, nil
}

// Addr returns the host:port string for the HTTP listener.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// SafeLogFields returns a map of config values that are safe to include in
// startup logs. Sensitive values (DSN, secrets, key paths) are omitted.
func (c *Config) SafeLogFields() map[string]any {
	return map[string]any{
		"server.host":                     c.Server.Host,
		"server.port":                     c.Server.Port,
		"session.inactivity_timeout_secs": c.Session.InactivityTimeoutSeconds,
		"migrate.path":                    c.Migrate.Path,
		"log_level":                       c.LogLevel,
	}
}

// envOr returns the environment variable named key, or fallback if not set.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt returns the environment variable named key as an integer, or
// defaultVal if not set. Returns an error if set but not parseable.
func envInt(key string, defaultVal int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, errors.New("must be an integer")
	}
	return n, nil
}
