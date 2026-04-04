package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms/internal/apperr"
	"lms/internal/domain/reports"
	"lms/internal/model"
)

// ReportsRepo implements reports.Repository against the lms schema.
type ReportsRepo struct {
	pool *pgxpool.Pool
}

// NewReportsRepo creates a new ReportsRepo.
func NewReportsRepo(pool *pgxpool.Pool) *ReportsRepo {
	return &ReportsRepo{pool: pool}
}

var _ reports.Repository = (*ReportsRepo)(nil)

// ── Definitions ───────────────────────────────────────────────────────────────

func (r *ReportsRepo) ListDefinitions(ctx context.Context) ([]*model.ReportDefinition, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, description, query_template, metric_aliases, default_filters, is_active, created_at, updated_at
		FROM lms.report_definitions
		WHERE is_active = TRUE
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var defs []*model.ReportDefinition
	for rows.Next() {
		d := &model.ReportDefinition{}
		var aliasesRaw, filtersRaw []byte
		if err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.QueryTemplate,
			&aliasesRaw, &filtersRaw, &d.IsActive, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		if aliasesRaw != nil {
			d.MetricAliases = json.RawMessage(aliasesRaw)
		}
		if filtersRaw != nil {
			d.DefaultFilters = json.RawMessage(filtersRaw)
		}
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

func (r *ReportsRepo) GetDefinition(ctx context.Context, id string) (*model.ReportDefinition, error) {
	d := &model.ReportDefinition{}
	var aliasesRaw, filtersRaw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, name, description, query_template, metric_aliases, default_filters, is_active, created_at, updated_at
		FROM lms.report_definitions WHERE id = $1`, id).
		Scan(&d.ID, &d.Name, &d.Description, &d.QueryTemplate,
			&aliasesRaw, &filtersRaw, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apperr.NotFound{Resource: "report_definition", ID: id}
	}
	if err != nil {
		return nil, err
	}
	if aliasesRaw != nil {
		d.MetricAliases = json.RawMessage(aliasesRaw)
	}
	if filtersRaw != nil {
		d.DefaultFilters = json.RawMessage(filtersRaw)
	}
	return d, nil
}

// ── Aggregates ────────────────────────────────────────────────────────────────

func (r *ReportsRepo) GetAggregate(ctx context.Context, definitionID, branchID, periodStart, periodEnd string) (*model.ReportAggregate, error) {
	agg := &model.ReportAggregate{}
	var dataRaw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, report_definition_id::text, branch_id::text, period_start::text, period_end::text,
		       aggregate_data, generated_at
		FROM lms.report_aggregates
		WHERE report_definition_id = $1
		  AND (branch_id = $2 OR ($2 = '' AND branch_id IS NULL))
		  AND period_start = $3::date
		  AND period_end   = $4::date`,
		definitionID, branchID, periodStart, periodEnd).
		Scan(&agg.ID, &agg.ReportDefinitionID, &agg.BranchID,
			&agg.PeriodStart, &agg.PeriodEnd, &dataRaw, &agg.GeneratedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &apperr.NotFound{Resource: "report_aggregate"}
	}
	if err != nil {
		return nil, err
	}
	if dataRaw != nil {
		agg.AggregateData = json.RawMessage(dataRaw)
	}
	return agg, nil
}

func (r *ReportsRepo) UpsertAggregate(ctx context.Context, agg *model.ReportAggregate) error {
	dataJSON, err := json.Marshal(agg.AggregateData)
	if err != nil {
		return err
	}
	var branchParam any
	if agg.BranchID != nil && *agg.BranchID != "" {
		branchParam = *agg.BranchID
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO lms.report_aggregates
		    (report_definition_id, branch_id, period_start, period_end, aggregate_data)
		VALUES ($1, $2, $3::date, $4::date, $5)
		ON CONFLICT (report_definition_id, branch_id, period_start, period_end)
		DO UPDATE SET aggregate_data = EXCLUDED.aggregate_data, generated_at = NOW()
		RETURNING id::text, generated_at`,
		agg.ReportDefinitionID, branchParam, agg.PeriodStart, agg.PeriodEnd, dataJSON).
		Scan(&agg.ID, &agg.GeneratedAt)
}

func (r *ReportsRepo) ListAggregates(ctx context.Context, branchID, definitionID string, from, to time.Time) ([]*model.ReportAggregate, error) {
	query := `
		SELECT id::text, report_definition_id::text, branch_id::text, period_start::text, period_end::text,
		       aggregate_data, generated_at
		FROM lms.report_aggregates
		WHERE period_start >= $1::date AND period_end <= $2::date`
	args := []any{from.Format("2006-01-02"), to.Format("2006-01-02")}
	i := 3

	if branchID != "" {
		query += fmt.Sprintf(" AND branch_id = $%d", i)
		args = append(args, branchID)
		i++
	}
	if definitionID != "" {
		query += fmt.Sprintf(" AND report_definition_id = $%d", i)
		args = append(args, definitionID)
	}
	query += " ORDER BY period_start DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aggs []*model.ReportAggregate
	for rows.Next() {
		agg := &model.ReportAggregate{}
		var dataRaw []byte
		if err := rows.Scan(&agg.ID, &agg.ReportDefinitionID, &agg.BranchID,
			&agg.PeriodStart, &agg.PeriodEnd, &dataRaw, &agg.GeneratedAt); err != nil {
			return nil, err
		}
		if dataRaw != nil {
			agg.AggregateData = json.RawMessage(dataRaw)
		}
		aggs = append(aggs, agg)
	}
	return aggs, rows.Err()
}

// ── Live queries ──────────────────────────────────────────────────────────────
// RunLiveQuery dispatches on queryTemplate (the value stored in report_definitions.query_template).
// Raw SQL is never stored in the DB and never executed from the DB — only the key is stored.
// Each case maps to a pre-defined, parameterised query built in Go.
//
// branchID contract:
//   - Non-empty: scopes the query to a single branch (all user-facing calls).
//   - Empty (""):  runs the query across all branches (no branch filter applied).
//     This is intentional and is ONLY called by the nightly scheduler in main.go
//     when producing cross-branch aggregate rows. It must never be reachable from
//     user-facing HTTP handlers — the reports.Service.RunReport method validates
//     that branchID is non-empty before calling this function.
func (r *ReportsRepo) RunLiveQuery(ctx context.Context, branchID, queryTemplate string, filters map[string]string, from, to time.Time) ([]map[string]any, error) {
	switch queryTemplate {
	case "circulation":
		return r.queryCirculation(ctx, branchID, from, to, filters)
	case "utilization":
		return r.queryUtilization(ctx, branchID, from, to)
	case "enrollment_mix":
		return r.queryEnrollmentMix(ctx, branchID, from, to)
	case "resource_yield":
		return r.queryResourceYield(ctx, branchID, from, to)
	case "reader_activity":
		return r.queryReaderActivity(ctx, branchID, from, to)
	case "feedback_summary":
		return r.queryFeedbackSummary(ctx, branchID, from, to)
	default:
		return nil, &apperr.Validation{Field: "query_template", Message: fmt.Sprintf("unknown template: %q", queryTemplate)}
	}
}

// queryCirculation counts circulation events grouped by event_type and holding category.
// Canonical: circulation | Hospitality aliases: n/a (native LMS metric)
func (r *ReportsRepo) queryCirculation(ctx context.Context, branchID string, from, to time.Time, filters map[string]string) ([]map[string]any, error) {
	categoryFilter := filters["category"]
	args := []any{from, to}
	where := `WHERE ce.created_at BETWEEN $1 AND $2`
	if categoryFilter != "" {
		where += " AND h.category = $3"
		args = append(args, categoryFilter)
	}
	if branchID != "" {
		argPos := len(args) + 1
		where += fmt.Sprintf(" AND ce.branch_id = $%d", argPos)
		args = append(args, branchID)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
		  ce.event_type,
		  COALESCE(h.category, 'unknown') AS holding_category,
		  COUNT(*)::int                    AS event_count
		FROM lms.circulation_events ce
		JOIN lms.copies               co ON co.id = ce.copy_id
		JOIN lms.holdings             h  ON h.id  = co.holding_id
		`+where+`
		GROUP BY ce.event_type, h.category
		ORDER BY ce.event_type, event_count DESC`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStringMaps(rows, "event_type", "holding_category", "event_count")
}

// queryUtilization computes the slot utilisation rate (= occupancy_rate) per copy status.
// slot_utilization_rate = checked_out_copies / total_copies
func (r *ReportsRepo) queryUtilization(ctx context.Context, branchID string, from, to time.Time) ([]map[string]any, error) {
	query := `
		SELECT
		  status_code                               AS copy_status,
		  COUNT(*)::int                             AS copy_count,
		  COUNT(*) FILTER (WHERE status_code = 'checked_out')::int AS checked_out_count,
		  ROUND(
		    COUNT(*) FILTER (WHERE status_code = 'checked_out')::numeric
		    / NULLIF(COUNT(*), 0) * 100, 2
		  )                                         AS slot_utilization_rate
		FROM lms.copies
		WHERE created_at <= $1
		GROUP BY status_code
		ORDER BY copy_count DESC`
	args := []any{to}
	if branchID != "" {
		query = `
		SELECT
		  status_code                               AS copy_status,
		  COUNT(*)::int                             AS copy_count,
		  COUNT(*) FILTER (WHERE status_code = 'checked_out')::int AS checked_out_count,
		  ROUND(
		    COUNT(*) FILTER (WHERE status_code = 'checked_out')::numeric
		    / NULLIF(COUNT(*), 0) * 100, 2
		  )                                         AS slot_utilization_rate
		FROM lms.copies
		WHERE branch_id = $1
		  AND created_at <= $2
		GROUP BY status_code
		ORDER BY copy_count DESC`
		args = []any{branchID, to}
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStringMaps(rows, "copy_status", "copy_count", "checked_out_count", "slot_utilization_rate")
}

// queryEnrollmentMix returns enrollment counts broken down by program category (= revenue_mix),
// venue_type (= room_type), and enrollment_channel (= channel).
func (r *ReportsRepo) queryEnrollmentMix(ctx context.Context, branchID string, from, to time.Time) ([]map[string]any, error) {
	query := `
		SELECT
		  COALESCE(p.category, 'uncategorised')  AS program_category,
		  COALESCE(p.venue_type, 'unspecified')  AS venue_type,
		  COALESCE(e.enrollment_channel, 'any')  AS enrollment_channel,
		  COUNT(*)::int                           AS enrollment_count,
		  COUNT(*) FILTER (WHERE e.status = 'confirmed')::int AS confirmed_count,
		  COUNT(*) FILTER (WHERE e.status = 'waitlisted')::int AS waitlisted_count
		FROM lms.enrollments e
		JOIN lms.programs p ON p.id = e.program_id
		WHERE e.enrolled_at BETWEEN $1 AND $2
		GROUP BY p.category, p.venue_type, e.enrollment_channel
		ORDER BY enrollment_count DESC`
	args := []any{from, to}
	if branchID != "" {
		query = `
		SELECT
		  COALESCE(p.category, 'uncategorised')  AS program_category,
		  COALESCE(p.venue_type, 'unspecified')  AS venue_type,
		  COALESCE(e.enrollment_channel, 'any')  AS enrollment_channel,
		  COUNT(*)::int                           AS enrollment_count,
		  COUNT(*) FILTER (WHERE e.status = 'confirmed')::int AS confirmed_count,
		  COUNT(*) FILTER (WHERE e.status = 'waitlisted')::int AS waitlisted_count
		FROM lms.enrollments e
		JOIN lms.programs p ON p.id = e.program_id
		WHERE e.branch_id = $1
		  AND e.enrolled_at BETWEEN $2 AND $3
		GROUP BY p.category, p.venue_type, e.enrollment_channel
		ORDER BY enrollment_count DESC`
		args = []any{branchID, from, to}
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStringMaps(rows,
		"program_category", "venue_type", "enrollment_channel",
		"enrollment_count", "confirmed_count", "waitlisted_count")
}

// queryResourceYield computes resource_yield_per_available_slot (= revpar):
// total confirmed enrollments / (sum of program capacities in the period).
// Expressed as a per-program breakdown for granularity.
func (r *ReportsRepo) queryResourceYield(ctx context.Context, branchID string, from, to time.Time) ([]map[string]any, error) {
	query := `
		SELECT
		  p.id::text                         AS program_id,
		  p.title                            AS program_title,
		  COALESCE(p.venue_type, 'unspecified') AS venue_type,
		  p.capacity::int                    AS capacity,
		  COUNT(e.id) FILTER (WHERE e.status = 'confirmed')::int AS confirmed_enrollments,
		  ROUND(
		    COUNT(e.id) FILTER (WHERE e.status = 'confirmed')::numeric
		    / NULLIF(p.capacity, 0) * 100, 2
		  )                                  AS resource_yield_per_available_slot
		FROM lms.programs p
		LEFT JOIN lms.enrollments e ON e.program_id = p.id
		WHERE p.starts_at BETWEEN $1 AND $2
		GROUP BY p.id, p.title, p.venue_type, p.capacity
		ORDER BY resource_yield_per_available_slot DESC NULLS LAST`
	args := []any{from, to}
	if branchID != "" {
		query = `
		SELECT
		  p.id::text                         AS program_id,
		  p.title                            AS program_title,
		  COALESCE(p.venue_type, 'unspecified') AS venue_type,
		  p.capacity::int                    AS capacity,
		  COUNT(e.id) FILTER (WHERE e.status = 'confirmed')::int AS confirmed_enrollments,
		  ROUND(
		    COUNT(e.id) FILTER (WHERE e.status = 'confirmed')::numeric
		    / NULLIF(p.capacity, 0) * 100, 2
		  )                                  AS resource_yield_per_available_slot
		FROM lms.programs p
		LEFT JOIN lms.enrollments e ON e.program_id = p.id AND e.branch_id = $1
		WHERE p.branch_id = $1
		  AND p.starts_at BETWEEN $2 AND $3
		GROUP BY p.id, p.title, p.venue_type, p.capacity
		ORDER BY resource_yield_per_available_slot DESC NULLS LAST`
		args = []any{branchID, from, to}
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStringMaps(rows,
		"program_id", "program_title", "venue_type", "capacity",
		"confirmed_enrollments", "resource_yield_per_available_slot")
}

// queryReaderActivity returns new registrations and active readers in the period.
func (r *ReportsRepo) queryReaderActivity(ctx context.Context, branchID string, from, to time.Time) ([]map[string]any, error) {
	query := `
		SELECT
		  'new_registrations'      AS metric,
		  COUNT(*)::int            AS value
		FROM lms.readers
		WHERE created_at BETWEEN $1 AND $2
		  AND status_code = 'active'
		UNION ALL
		SELECT
		  'active_borrowers',
		  COUNT(DISTINCT reader_id)::int
		FROM lms.circulation_events
		WHERE created_at BETWEEN $1 AND $2
		  AND event_type = 'checkout'
		UNION ALL
		SELECT
		  'active_enrollments',
		  COUNT(DISTINCT reader_id)::int
		FROM lms.enrollments
		WHERE enrolled_at BETWEEN $1 AND $2
		  AND status IN ('confirmed', 'pending')`
	args := []any{from, to}
	if branchID != "" {
		query = `
		SELECT
		  'new_registrations'      AS metric,
		  COUNT(*)::int            AS value
		FROM lms.readers
		WHERE branch_id = $1 AND created_at BETWEEN $2 AND $3
		  AND status_code = 'active'
		UNION ALL
		SELECT
		  'active_borrowers',
		  COUNT(DISTINCT reader_id)::int
		FROM lms.circulation_events
		WHERE branch_id = $1 AND created_at BETWEEN $2 AND $3
		  AND event_type = 'checkout'
		UNION ALL
		SELECT
		  'active_enrollments',
		  COUNT(DISTINCT reader_id)::int
		FROM lms.enrollments
		WHERE branch_id = $1 AND enrolled_at BETWEEN $2 AND $3
		  AND status IN ('confirmed', 'pending')`
		args = []any{branchID, from, to}
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStringMaps(rows, "metric", "value")
}

// queryFeedbackSummary aggregates feedback ratings and counts by target_type and status.
func (r *ReportsRepo) queryFeedbackSummary(ctx context.Context, branchID string, from, to time.Time) ([]map[string]any, error) {
	query := `
		SELECT
		  target_type,
		  status,
		  COUNT(*)::int             AS feedback_count,
		  ROUND(AVG(rating), 2)     AS avg_rating,
		  COUNT(*) FILTER (WHERE rating = 5)::int AS five_star_count,
		  COUNT(*) FILTER (WHERE rating <= 2)::int AS low_rating_count
		FROM lms.feedback
		WHERE submitted_at BETWEEN $1 AND $2
		GROUP BY target_type, status
		ORDER BY target_type, status`
	args := []any{from, to}
	if branchID != "" {
		query = `
		SELECT
		  target_type,
		  status,
		  COUNT(*)::int             AS feedback_count,
		  ROUND(AVG(rating), 2)     AS avg_rating,
		  COUNT(*) FILTER (WHERE rating = 5)::int AS five_star_count,
		  COUNT(*) FILTER (WHERE rating <= 2)::int AS low_rating_count
		FROM lms.feedback
		WHERE branch_id = $1
		  AND submitted_at BETWEEN $2 AND $3
		GROUP BY target_type, status
		ORDER BY target_type, status`
		args = []any{branchID, from, to}
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStringMaps(rows,
		"target_type", "status", "feedback_count",
		"avg_rating", "five_star_count", "low_rating_count")
}

// scanStringMaps scans query rows into []map[string]any using the provided column names.
func scanStringMaps(rows pgx.Rows, cols ...string) ([]map[string]any, error) {
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = vals[i]
		}
		result = append(result, row)
	}
	if result == nil {
		result = []map[string]any{}
	}
	return result, rows.Err()
}
