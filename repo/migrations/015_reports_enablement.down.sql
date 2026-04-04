SET search_path = lms, public;

-- Remove newly added report definitions.
DELETE FROM report_definitions
WHERE name IN ('resource_yield', 'circulation_overview', 'reader_activity', 'feedback_summary');

-- Revert updated seeded definitions to their original migration-014 state.
UPDATE report_definitions
SET
  description = 'Program slot utilization by venue type and enrollment channel',
  query_template = 'SELECT p.venue_type, p.enrollment_channel, p.capacity,
                COUNT(e.id) FILTER (WHERE e.status = ''confirmed'') AS confirmed_count,
                ROUND(COUNT(e.id) FILTER (WHERE e.status = ''confirmed'')::numeric / NULLIF(p.capacity, 0) * 100, 2) AS slot_utilization_rate
         FROM lms.programs p
         LEFT JOIN lms.enrollments e ON e.program_id = p.id
         WHERE p.branch_id = :branch_id
           AND p.starts_at >= :period_start AND p.starts_at < :period_end
         GROUP BY p.venue_type, p.enrollment_channel, p.capacity',
  metric_aliases = '{
    "slot_utilization_rate": "occupancy_rate",
    "venue_type": "room_type",
    "enrollment_channel": "channel"
  }'::jsonb,
  default_filters = NULL,
  updated_at = NOW()
WHERE name = 'program_utilization';

UPDATE report_definitions
SET
  description = 'Enrollment distribution by program category (revenue mix analog)',
  query_template = 'SELECT p.category,
                COUNT(e.id) AS enrollment_count,
                ROUND(COUNT(e.id)::numeric / NULLIF(SUM(COUNT(e.id)) OVER (), 0) * 100, 2) AS enrollment_mix_by_category
         FROM lms.programs p
         JOIN lms.enrollments e ON e.program_id = p.id
         WHERE p.branch_id = :branch_id
           AND p.starts_at >= :period_start AND p.starts_at < :period_end
         GROUP BY p.category',
  metric_aliases = '{"enrollment_mix_by_category": "revenue_mix"}'::jsonb,
  default_filters = NULL,
  updated_at = NOW()
WHERE name = 'enrollment_mix';

-- Remove reports:admin from the admin role and permissions table.
DELETE FROM role_permissions
WHERE role_id = '11111111-0000-0000-0000-000000000001'
  AND permission_id = '22220013-0000-0000-0000-000000000003';

DELETE FROM permissions
WHERE id = '22220013-0000-0000-0000-000000000003'
  AND name = 'reports:admin';
