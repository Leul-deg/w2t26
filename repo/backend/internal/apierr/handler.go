// Package apierr provides the custom Echo HTTP error handler that maps
// application error types to appropriate HTTP status codes and response bodies.
// Internal error details are never exposed to clients.
package apierr

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
)

// ErrorHandler returns an Echo-compatible HTTP error handler that maps
// application error types to JSON responses. Use as:
//
//	e.HTTPErrorHandler = apierr.ErrorHandler
func ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	var body any

	switch e := err.(type) {
	case *apperr.Unauthorized:
		code = http.StatusUnauthorized
		body = map[string]string{
			"error": "unauthenticated",
		}

	case *apperr.Forbidden:
		code = http.StatusForbidden
		body = map[string]string{
			"error":  "forbidden",
			"detail": e.Error(),
		}

	case *apperr.NotFound:
		code = http.StatusNotFound
		body = map[string]string{
			"error":  "not_found",
			"detail": e.Error(),
		}

	case *apperr.Conflict:
		code = http.StatusConflict
		body = map[string]string{
			"error":  "conflict",
			"detail": e.Message,
		}

	case *apperr.Validation:
		code = http.StatusUnprocessableEntity
		body = map[string]string{
			"error":  "validation_error",
			"field":  e.Field,
			"detail": e.Message,
		}

	case *apperr.AccountLocked:
		code = http.StatusLocked // 423
		body = map[string]any{
			"error":               "account_locked",
			"retry_after_seconds": e.SecondsRemaining,
		}

	case *apperr.CaptchaRequired:
		code = http.StatusPreconditionRequired // 428
		// Extract the challenge question from the key (key:question format).
		challengeKey, question := parseChallengeKey(e.ChallengeKey)
		body = map[string]string{
			"error":         "captcha_required",
			"challenge_key": challengeKey,
			"challenge":     question,
		}

	case *apperr.Unimplemented:
		code = http.StatusNotImplemented
		body = map[string]string{
			"error": "not_implemented",
		}

	case *apperr.Internal:
		// Log the real cause but never expose it to the client.
		slog.Error("internal server error",
			"error", e.Cause,
			"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
		)
		code = http.StatusInternalServerError
		body = map[string]string{
			"error": "internal_server_error",
		}

	case *echo.HTTPError:
		code = e.Code
		if msg, ok := e.Message.(string); ok {
			body = map[string]string{"error": msg}
		} else {
			body = map[string]any{"error": e.Message}
		}

	default:
		// Unknown error type — log the real cause but return a generic message.
		slog.Error("unhandled error",
			"error", err,
			"type", err,
			"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
		)
		code = http.StatusInternalServerError
		body = map[string]string{
			"error": "internal_server_error",
		}
	}

	// Send the response. Ignore encoding errors since the connection may have
	// already been closed or the response partially written.
	if c.Request().Method == http.MethodHead {
		_ = c.NoContent(code)
	} else {
		_ = c.JSON(code, body)
	}
}

// parseChallengeKey splits a challenge key of the form "key:question" into
// its two parts. If no ':' is present, returns the full key and an empty question.
func parseChallengeKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}
