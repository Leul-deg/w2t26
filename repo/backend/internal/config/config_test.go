package config_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/internal/config"
)

// clearEnv removes all config-related environment variables so each test
// starts from a known-blank state. t.Setenv automatically restores original
// values when the test finishes.
func clearEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"DATABASE_URL",
		"CRYPTO_KEY_FILE",
		"SERVER_HOST",
		"SERVER_PORT",
		"SESSION_INACTIVITY_SECONDS",
		"MIGRATIONS_PATH",
		"LOG_LEVEL",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}
}

func TestLoad_MissingRequiredFields_ReturnsError(t *testing.T) {
	clearEnv(t)

	_, err := config.Load()

	require.Error(t, err, "Load must fail when required fields are missing")
	assert.Contains(t, err.Error(), "DATABASE_URL")
	assert.Contains(t, err.Error(), "CRYPTO_KEY_FILE")
}

func TestLoad_ValidMinimalConfig_Succeeds(t *testing.T) {
	clearEnv(t)

	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb?sslmode=disable")
	t.Setenv("CRYPTO_KEY_FILE", "/tmp/test.key")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 1800, cfg.Session.InactivityTimeoutSeconds)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoad_CustomPort_Parsed(t *testing.T) {
	clearEnv(t)

	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb?sslmode=disable")
	t.Setenv("CRYPTO_KEY_FILE", "/tmp/test.key")
	t.Setenv("SERVER_PORT", "9090")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
}

func TestLoad_InvalidPort_ReturnsError(t *testing.T) {
	clearEnv(t)

	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb?sslmode=disable")
	t.Setenv("CRYPTO_KEY_FILE", "/tmp/test.key")
	t.Setenv("SERVER_PORT", "not-a-number")

	_, err := config.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SERVER_PORT")
}

func TestConfig_Addr_Format(t *testing.T) {
	clearEnv(t)

	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb?sslmode=disable")
	t.Setenv("CRYPTO_KEY_FILE", "/tmp/test.key")
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "8888")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1:8888", cfg.Addr())
}

// TestLoad_SafeLogFields_OmitsSensitiveValues verifies that SafeLogFields does
// not expose database credentials or key paths.
func TestLoad_SafeLogFields_OmitsSensitiveValues(t *testing.T) {
	clearEnv(t)

	dsn := "postgres://adminuser:topsecret@dbhost:5432/lms?sslmode=require"

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("CRYPTO_KEY_FILE", "/secrets/lms.key")

	cfg, err := config.Load()
	require.NoError(t, err)

	fields := cfg.SafeLogFields()

	for k, v := range fields {
		serialized := fmt.Sprintf("%v", v)
		assert.NotContains(t, serialized, "topsecret",
			"SafeLogFields field %q must not contain the DB password", k)
	}

	// Non-sensitive fields must be present.
	assert.Contains(t, fields, "server.port")
	assert.Contains(t, fields, "server.host")
}
