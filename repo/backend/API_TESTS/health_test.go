package apitests

// health_test.go tests GET /api/v1/health (liveness) and GET /api/v1/ready
// (readiness). Both endpoints require no authentication.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealth_Liveness verifies that GET /api/v1/health returns 200 with
// status "ok" and requires no authentication cookie.
func TestHealth_Liveness(t *testing.T) {
	app := newCompleteTestApp(t)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/health", nil)
	assert.Equal(t, http.StatusOK, rec.Code,
		"liveness check must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"], "liveness response must include status=ok")
}

// TestHealth_Readiness verifies that GET /api/v1/ready returns 200 when the
// test database is reachable.
func TestHealth_Readiness(t *testing.T) {
	app := newCompleteTestApp(t)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/ready", nil)
	assert.Equal(t, http.StatusOK, rec.Code,
		"readiness check must return 200 when DB is up: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"], "readiness response must include status=ok")
}

// TestHealth_NoAuthRequired verifies that health endpoints do not redirect to
// login or return 401/403 when called without a session cookie.
func TestHealth_NoAuthRequired(t *testing.T) {
	app := newCompleteTestApp(t)

	for _, path := range []string{"/api/v1/health", "/api/v1/ready"} {
		t.Run(path, func(t *testing.T) {
			rec := doRequest(t, app.testApp, http.MethodGet, path, nil)
			assert.NotEqual(t, http.StatusUnauthorized, rec.Code,
				"%s must not require auth (got 401)", path)
			assert.NotEqual(t, http.StatusForbidden, rec.Code,
				"%s must not require auth (got 403)", path)
		})
	}
}
