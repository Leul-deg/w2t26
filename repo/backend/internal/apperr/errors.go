// Package apperr defines application-level error types used across all domain
// packages. Each type maps to a specific HTTP status code in the error handler
// middleware. Using typed errors (not string matching) keeps the boundary clean.
package apperr

import "fmt"

// NotFound is returned when a resource cannot be found, including when branch
// scoping hides it from the caller. Do not distinguish "not found" from
// "found but access denied" in responses — return 404 for both to prevent
// enumeration attacks.
type NotFound struct {
	Resource string
	ID       string
}

func (e *NotFound) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
	}
	return fmt.Sprintf("%s not found", e.Resource)
}

// Validation is returned when input fails business-rule or format validation.
// Field may be empty for entity-level (cross-field) errors.
type Validation struct {
	Field   string
	Message string
}

func (e *Validation) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error — %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// Conflict is returned for uniqueness violations or state machine violations
// (e.g. enrolling in a full program, duplicate barcode).
type Conflict struct {
	Resource string
	Message  string
}

func (e *Conflict) Error() string {
	return fmt.Sprintf("conflict on %s: %s", e.Resource, e.Message)
}

// Forbidden is returned when the caller is authenticated but lacks permission
// for the requested action. Do not include the reason for the denial.
type Forbidden struct {
	Action   string
	Resource string
}

func (e *Forbidden) Error() string {
	return fmt.Sprintf("forbidden: %s %s", e.Action, e.Resource)
}

// Unauthorized is returned when the caller is not authenticated (no valid session).
type Unauthorized struct{}

func (e *Unauthorized) Error() string {
	return "authentication required"
}

// AccountLocked is returned when a user account is locked due to failed attempts.
type AccountLocked struct {
	SecondsRemaining int
}

func (e *AccountLocked) Error() string {
	return fmt.Sprintf("account locked for %d more seconds", e.SecondsRemaining)
}

// CaptchaRequired is returned when a CAPTCHA challenge must be solved before retrying.
type CaptchaRequired struct {
	ChallengeKey string
}

func (e *CaptchaRequired) Error() string {
	return "CAPTCHA required before next login attempt"
}

// Internal is returned for unexpected server-side failures. The detail is
// logged server-side but a generic message is returned to the client.
type Internal struct {
	Cause error
}

func (e *Internal) Error() string {
	return "an internal error occurred"
}

func (e *Internal) Unwrap() error {
	return e.Cause
}

// Unimplemented is returned by scaffold handlers that are not yet implemented.
type Unimplemented struct {
	Feature string
}

func (e *Unimplemented) Error() string {
	return fmt.Sprintf("%s is not yet implemented", e.Feature)
}
