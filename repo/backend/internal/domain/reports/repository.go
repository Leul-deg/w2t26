// Package reports provides configurable analytics over the LMS data model.
//
// KPI / hospitality-term handling:
//   - occupancy_rate             → slot_utilization_rate   (checked-out copies / total copies)
//   - revpar                     → resource_yield_per_available_slot (enrollments / capacity × program count)
//   - revenue_mix                → enrollment_mix_by_category  (enrollments by program.category)
//   - room_type                  → venue_type              (programs.venue_type column)
//   - channel                    → enrollment_channel      (programs.enrollment_channel column)
//
// Both the canonical name and the hospitality alias are supported in
// filters and in the metric_aliases JSONB column of report_definitions.
package reports

import (
	"context"
	"time"

	"lms/internal/model"
)

// Repository is the data-access contract for the reports domain.
type Repository interface {
	// Definitions
	ListDefinitions(ctx context.Context) ([]*model.ReportDefinition, error)
	GetDefinition(ctx context.Context, id string) (*model.ReportDefinition, error)

	// Aggregates (pre-computed cache)
	GetAggregate(ctx context.Context, definitionID, branchID, periodStart, periodEnd string) (*model.ReportAggregate, error)
	UpsertAggregate(ctx context.Context, agg *model.ReportAggregate) error
	ListAggregates(ctx context.Context, branchID, definitionID string, from, to time.Time) ([]*model.ReportAggregate, error)

	// Live queries — dispatched by query_template key stored in report_definitions.
	// Returns rows as []map[string]any so each report type can shape its own response.
	RunLiveQuery(ctx context.Context, branchID, queryTemplate string, filters map[string]string, from, to time.Time) ([]map[string]any, error)
}
