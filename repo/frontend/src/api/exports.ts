// API client for the audited export pipeline.

import { apiClient } from './client';
import type { PageResult } from './readers';

// ── Types ─────────────────────────────────────────────────────────────────────

export type ExportType = 'readers' | 'holdings' | 'copies' | 'circulation' | 'programs' | 'enrollments' | 'audit_events' | 'report';

export interface ExportJob {
  id: string;
  branch_id: string;
  export_type: ExportType;
  filters_applied?: Record<string, string>;
  row_count?: number;
  file_name?: string;
  exported_by: string;
  exported_at: string;
}

// ── API calls ─────────────────────────────────────────────────────────────────

function list(params?: { page?: number; per_page?: number }): Promise<PageResult<ExportJob>> {
  const qs = new URLSearchParams();
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PageResult<ExportJob>>(`/exports${q ? '?' + q : ''}`);
}

// triggerExport sends POST and expects a CSV blob back.
// Returns { blob, fileName, jobId } so the caller can trigger a download.
async function triggerExport(
  exportType: 'readers' | 'holdings',
): Promise<{ blob: Blob; fileName: string; jobId: string }> {
  const res = await fetch(`/api/v1/exports/${exportType}`, {
    method: 'POST',
    credentials: 'include',
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: 'unknown' }));
    throw Object.assign(new Error(data.detail ?? data.error ?? 'Export failed'), {
      status: res.status,
      body: data,
    });
  }
  const blob = await res.blob();
  const disposition = res.headers.get('Content-Disposition') ?? '';
  const match = disposition.match(/filename="([^"]+)"/);
  const fileName = match ? match[1] : `${exportType}_export.csv`;
  const jobId = res.headers.get('X-Export-Job-ID') ?? '';
  return { blob, fileName, jobId };
}

export const exportsApi = {
  list,
  triggerExport,
};
