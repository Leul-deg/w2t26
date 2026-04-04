// Package audit provides the audit event logger.
// All audit writes must go through this package.
// Direct writes to the audit_events table from other packages are prohibited.
package audit

import (
	"context"

	"lms/internal/model"
)

// Repository defines the data access contract for the audit log.
// It exposes only Insert and query methods — no Update or Delete.
type Repository interface {
	// Insert appends a new audit event. Never returns a conflict error because
	// each event has a fresh UUID primary key.
	Insert(ctx context.Context, e *model.AuditEvent) error

	// List returns a paginated, time-ordered slice of audit events.
	// Filters are applied server-side; all parameters are optional.
	List(ctx context.Context, f AuditFilter, p model.Pagination) (model.PageResult[*model.AuditEvent], error)
}

// AuditFilter carries optional filter parameters for audit log queries.
type AuditFilter struct {
	EventType    *string
	ActorUserID  *string
	BranchID     *string
	ResourceType *string
	ResourceID   *string
	FromTime     *string // RFC3339
	ToTime       *string // RFC3339
}
