package model

import "time"

// ReportDefinition is a named, parameterised report configuration.
// MetricAliases maps canonical metric names to their hospitality-origin aliases
// so both terms remain queryable:
//   "slot_utilization_rate" → "occupancy_rate"
//   "resource_yield_per_available_slot" → "revpar"
//   "enrollment_mix_by_category" → "revenue_mix"
//   "venue_type" → "room_type"
//   "enrollment_channel" → "channel"
type ReportDefinition struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    *string   `json:"description,omitempty"`
	QueryTemplate  string    `json:"-"` // not exposed in API responses
	MetricAliases  any       `json:"metric_aliases,omitempty"` // JSONB
	DefaultFilters any       `json:"default_filters,omitempty"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ReportAggregate is a cached pre-computed result for a report definition
// over a specific branch and time period.
type ReportAggregate struct {
	ID                   string    `json:"id"`
	ReportDefinitionID   string    `json:"report_definition_id"`
	BranchID             *string   `json:"branch_id,omitempty"` // nil = all branches
	PeriodStart          string    `json:"period_start"`        // DATE as YYYY-MM-DD
	PeriodEnd            string    `json:"period_end"`
	AggregateData        any       `json:"aggregate_data"` // JSONB
	GeneratedAt          time.Time `json:"generated_at"`
}
