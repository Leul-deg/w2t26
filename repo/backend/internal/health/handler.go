// Package health provides the liveness and readiness endpoints.
// GET /api/v1/health  — always returns 200 if the process is running.
// GET /api/v1/ready   — returns 200 only if the database is reachable.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"lms/internal/db"
)

// Response is the JSON body returned by both health endpoints.
type Response struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	// Detail is populated only on the readiness endpoint.
	Detail string `json:"detail,omitempty"`
}

const version = "0.1.0"

// LiveHandler handles GET /api/v1/health.
// It returns 200 as long as the process is alive. No database check.
func LiveHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, Response{
		Status:  "ok",
		Version: version,
	})
}

// ReadyHandler handles GET /api/v1/ready.
// It pings the database and returns 200 only if the ping succeeds.
// This is useful for process supervisors and smoke tests.
func ReadyHandler(pool *db.Pool) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, Response{
				Status:  "degraded",
				Version: version,
				Detail:  "database unreachable",
			})
		}

		return c.JSON(http.StatusOK, Response{
			Status:  "ok",
			Version: version,
		})
	}
}
