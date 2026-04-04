import { HttpError, apiClient } from './client';

export interface ReportDefinition {
  id: string;
  name: string;
  description?: string;
  metric_aliases?: Record<string, string>;
  default_filters?: Record<string, string>;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface ReportAggregate {
  id: string;
  report_definition_id: string;
  branch_id?: string;
  period_start: string;
  period_end: string;
  aggregate_data: unknown;
  generated_at: string;
}

export interface ReportRunResult {
  definition: ReportDefinition;
  from: string;
  to: string;
  rows: Array<Record<string, unknown>>;
  row_count: number;
}

export interface ReportRecalcResult {
  aggregates_computed: number;
}

function listDefinitions(): Promise<ReportDefinition[]> {
  return apiClient.get<ReportDefinition[]>('/reports/definitions');
}

function runReport(params: {
  definition_id: string;
  from: string;
  to: string;
  branch_id?: string;
  filters?: Record<string, string>;
}): Promise<ReportRunResult> {
  const qs = new URLSearchParams({
    definition_id: params.definition_id,
    from: params.from,
    to: params.to,
  });
  if (params.branch_id?.trim()) qs.set('branch_id', params.branch_id.trim());
  for (const [key, value] of Object.entries(params.filters ?? {})) {
    if (value.trim()) qs.set(key, value);
  }
  return apiClient.get<ReportRunResult>(`/reports/run?${qs.toString()}`);
}

function listAggregates(params: {
  definition_id?: string;
  from: string;
  to: string;
  branch_id?: string;
}): Promise<ReportAggregate[]> {
  const qs = new URLSearchParams({ from: params.from, to: params.to });
  if (params.definition_id) qs.set('definition_id', params.definition_id);
  if (params.branch_id?.trim()) qs.set('branch_id', params.branch_id.trim());
  return apiClient.get<ReportAggregate[]>(`/reports/aggregates?${qs.toString()}`);
}

function recalculateAggregates(data: {
  definition_id?: string;
  from: string;
  to: string;
}): Promise<ReportRecalcResult> {
  return apiClient.post<ReportRecalcResult>('/reports/recalculate', data);
}

async function exportReport(params: {
  definition_id: string;
  from: string;
  to: string;
  branch_id?: string;
  filters?: Record<string, string>;
}): Promise<{ blob: Blob; fileName: string; exportJobId?: string }> {
  const qs = new URLSearchParams({
    definition_id: params.definition_id,
    from: params.from,
    to: params.to,
  });
  if (params.branch_id?.trim()) qs.set('branch_id', params.branch_id.trim());
  for (const [key, value] of Object.entries(params.filters ?? {})) {
    if (value.trim()) qs.set(key, value);
  }

  const res = await fetch(`/api/v1/reports/export?${qs.toString()}`, {
    method: 'GET',
    credentials: 'include',
    headers: { Accept: 'text/csv' },
  });

  if (!res.ok) {
    const body = await res
      .json()
      .catch(() => ({ error: 'export_failed', detail: 'failed to export report' }));
    throw new HttpError(res.status, body);
  }

  const blob = await res.blob();
  const contentDisposition = res.headers.get('Content-Disposition') ?? '';
  const match = /filename="([^"]+)"/.exec(contentDisposition);
  const fileName = match ? match[1] : `report_export_${params.from}_${params.to}.csv`;
  const exportJobId = res.headers.get('X-Export-Job-ID') ?? undefined;
  return { blob, fileName, exportJobId };
}

export const reportsApi = {
  listDefinitions,
  run: runReport,
  listAggregates,
  recalculate: recalculateAggregates,
  export: exportReport,
};
