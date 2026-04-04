package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/domain/audit"
	"lms/internal/model"
)

// AuditRepo implements audit.Repository against the lms schema.
// This table is append-only; no Update or Delete operations are exposed.
type AuditRepo struct {
	pool *pgxpool.Pool
}

// NewAuditRepo creates a new AuditRepo backed by the given connection pool.
func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

// Insert appends a new audit event. Uses a fresh UUID primary key so
// conflicts are impossible. Never returns an error from a unique constraint.
func (r *AuditRepo) Insert(ctx context.Context, e *model.AuditEvent) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO lms.audit_events
		 (event_type, actor_user_id, actor_username, workstation_id, ip_address,
		  branch_id, resource_type, resource_id, before_value, after_value, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id::text, created_at`,
		e.EventType,
		e.ActorUserID,
		e.ActorUsername,
		e.WorkstationID,
		e.IPAddress,
		e.BranchID,
		e.ResourceType,
		e.ResourceID,
		marshalJSON(e.BeforeValue),
		marshalJSON(e.AfterValue),
		marshalJSON(e.Metadata),
	).Scan(&e.ID, &e.CreatedAt)
	return err
}

// List returns a paginated, time-ordered (DESC) slice of audit events with optional filters.
func (r *AuditRepo) List(ctx context.Context, f audit.AuditFilter, p model.Pagination) (model.PageResult[*model.AuditEvent], error) {
	// Build the WHERE clause dynamically from the filter.
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	addFilter := func(col, val string) {
		where += fmt.Sprintf(" AND %s = $%d", col, argIdx)
		args = append(args, val)
		argIdx++
	}

	if f.EventType != nil {
		addFilter("event_type", *f.EventType)
	}
	if f.ActorUserID != nil {
		addFilter("actor_user_id::text", *f.ActorUserID)
	}
	if f.BranchID != nil {
		addFilter("branch_id::text", *f.BranchID)
	}
	if f.ResourceType != nil {
		addFilter("resource_type", *f.ResourceType)
	}
	if f.ResourceID != nil {
		addFilter("resource_id::text", *f.ResourceID)
	}
	if f.FromTime != nil {
		t, err := time.Parse(time.RFC3339, *f.FromTime)
		if err == nil {
			where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
			args = append(args, t)
			argIdx++
		}
	}
	if f.ToTime != nil {
		t, err := time.Parse(time.RFC3339, *f.ToTime)
		if err == nil {
			where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
			args = append(args, t)
			argIdx++
		}
	}

	// Count query
	countSQL := "SELECT COUNT(*) FROM lms.audit_events " + where
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return model.PageResult[*model.AuditEvent]{}, err
	}

	// Data query with pagination
	limitIdx := argIdx
	offsetIdx := argIdx + 1
	dataSQL := fmt.Sprintf(
		`SELECT id::text, event_type, actor_user_id::text, actor_username, workstation_id, ip_address::text,
		        branch_id::text, resource_type, resource_id::text, before_value, after_value, metadata, created_at
		 FROM lms.audit_events
		 %s
		 ORDER BY created_at DESC
		 LIMIT $%d OFFSET $%d`,
		where, limitIdx, offsetIdx,
	)
	dataArgs := append(args, p.Limit(), p.Offset())

	rows, err := r.pool.Query(ctx, dataSQL, dataArgs...)
	if err != nil {
		return model.PageResult[*model.AuditEvent]{}, err
	}
	defer rows.Close()

	var events []*model.AuditEvent
	for rows.Next() {
		e := &model.AuditEvent{}
		var beforeRaw, afterRaw, metaRaw []byte
		var ipText *string
		if err := rows.Scan(
			&e.ID, &e.EventType, &e.ActorUserID, &e.ActorUsername,
			&e.WorkstationID, &ipText, &e.BranchID,
			&e.ResourceType, &e.ResourceID,
			&beforeRaw, &afterRaw, &metaRaw,
			&e.CreatedAt,
		); err != nil {
			return model.PageResult[*model.AuditEvent]{}, err
		}
		if len(beforeRaw) > 0 {
			e.BeforeValue = string(beforeRaw)
		}
		if len(afterRaw) > 0 {
			e.AfterValue = string(afterRaw)
		}
		if len(metaRaw) > 0 {
			e.Metadata = string(metaRaw)
		}
		e.IPAddress = ipText
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return model.PageResult[*model.AuditEvent]{}, err
	}

	return model.NewPageResult(events, total, p), nil
}

// marshalJSON converts an arbitrary value to a JSON string for JSONB columns.
// Returns nil for nil/empty values.
func marshalJSON(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		return val
	}
	// For structured data we could use encoding/json here;
	// passing the value directly works for pgx with JSONB columns.
	return v
}
