SET search_path = lms, public;

-- Add reports:admin so administrators can trigger on-demand recalculation.
INSERT INTO permissions (id, name, description)
VALUES ('22220013-0000-0000-0000-000000000003', 'reports:admin', 'Recalculate report aggregates and manage reporting caches')
ON CONFLICT (id) DO NOTHING;

-- Ensure the administrator role receives the new permission.
INSERT INTO role_permissions (role_id, permission_id)
SELECT '11111111-0000-0000-0000-000000000001', id
FROM permissions
WHERE name = 'reports:admin'
ON CONFLICT DO NOTHING;

-- Normalize seeded report definitions so query_template stores the supported
-- internal template key rather than raw SQL text.
INSERT INTO report_definitions (name, description, query_template, metric_aliases, default_filters, is_active)
VALUES
  (
    'program_utilization',
    'Program slot utilization by venue type and enrollment channel',
    'utilization',
    '{
      "slot_utilization_rate": "occupancy_rate",
      "venue_type": "room_type",
      "enrollment_channel": "channel"
    }'::jsonb,
    '{"group_by":"copy_status"}'::jsonb,
    TRUE
  ),
  (
    'enrollment_mix',
    'Enrollment distribution by program category, venue type, and enrollment channel',
    'enrollment_mix',
    '{
      "enrollment_mix_by_category": "revenue_mix",
      "venue_type": "room_type",
      "enrollment_channel": "channel"
    }'::jsonb,
    '{"group_by":"program_category"}'::jsonb,
    TRUE
  ),
  (
    'resource_yield',
    'Program yield per available capacity slot (RevPAR analog)',
    'resource_yield',
    '{
      "resource_yield_per_available_slot": "revpar",
      "venue_type": "room_type"
    }'::jsonb,
    '{"group_by":"program_id"}'::jsonb,
    TRUE
  ),
  (
    'circulation_overview',
    'Circulation events by event type and holding category',
    'circulation',
    '{}'::jsonb,
    '{"group_by":"event_type"}'::jsonb,
    TRUE
  ),
  (
    'reader_activity',
    'Reader activity summary across registrations, borrowing, and enrollments',
    'reader_activity',
    '{}'::jsonb,
    '{"group_by":"metric"}'::jsonb,
    TRUE
  ),
  (
    'feedback_summary',
    'Feedback rating and moderation summary by target type',
    'feedback_summary',
    '{}'::jsonb,
    '{"group_by":"target_type"}'::jsonb,
    TRUE
  )
ON CONFLICT (name) DO UPDATE
SET description = EXCLUDED.description,
    query_template = EXCLUDED.query_template,
    metric_aliases = EXCLUDED.metric_aliases,
    default_filters = EXCLUDED.default_filters,
    is_active = EXCLUDED.is_active,
    updated_at = NOW();
