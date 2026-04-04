// Package middleware provides Echo middleware used across all routes.
package middleware

import (
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// RequestID returns Echo's built-in RequestID middleware configured for this
// application. The generated ID is attached to the request context and echoed
// back in the X-Request-ID response header for log correlation.
func RequestID() echo.MiddlewareFunc {
	return echomw.RequestIDWithConfig(echomw.RequestIDConfig{
		// Use Echo's default UUID generator.
		Generator: echomw.DefaultRequestIDConfig.Generator,
		// Expose the ID in the response header so callers can correlate logs.
		RequestIDHandler: func(c echo.Context, id string) {
			c.Response().Header().Set(echo.HeaderXRequestID, id)
		},
	})
}
