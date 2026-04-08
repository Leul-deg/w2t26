// API client for the bulk import pipeline.

import { apiClient } from './client';
import type { PageResult } from './readers';

// ── Types ─────────────────────────────────────────────────────────────────────

export type ImportType = 'readers' | 'holdings';

export type ImportStatus =
  | 'uploaded'
  | 'previewing'
  | 'preview_ready'
  | 'committed'
  | 'rolled_back'
  | 'failed';

export interface RowError {
  row: number;
  field: string;
  message: string;
}

export interface ImportJob {
  id: string;
  branch_id: string;
  import_type: ImportType;
  status: ImportStatus;
  file_name: string;
  row_count?: number;
  error_count: number;
  error_summary?: RowError[];
  valid_row_count: number;
  invalid_row_count: number;
  completeness_percent: number;
  completeness_threshold_percent: number;
  meets_completeness_threshold: boolean;
  uploaded_by: string;
  uploaded_at: string;
  committed_at?: string;
  rolled_back_at?: string;
}

export interface ImportRow {
  id: string;
  job_id: string;
  row_number: number;
  raw_data: Record<string, string>;
  parsed_data?: Record<string, unknown>;
  status: 'pending' | 'valid' | 'invalid' | 'committed' | 'rolled_back';
  error_details?: string;
  created_at: string;
}

export interface PreviewResponse {
  job: ImportJob;
  rows: PageResult<ImportRow>;
}

// ── API calls ─────────────────────────────────────────────────────────────────

async function uploadFile(importType: ImportType, file: File): Promise<ImportJob> {
  const form = new FormData();
  form.append('import_type', importType);
  form.append('file', file);

  const res = await fetch('/api/v1/imports', {
    method: 'POST',
    credentials: 'include',
    body: form,
  });

  const data = await res.json().catch(() => ({ error: 'parse_error' }));
  // 202 = preview_ready, 422 = validation errors present — both return the job.
  if (res.status === 202 || res.status === 422) {
    return data as ImportJob;
  }
  throw Object.assign(new Error(data.detail ?? data.error ?? 'Upload failed'), {
    status: res.status,
    body: data,
  });
}

function list(params?: { page?: number; per_page?: number }): Promise<PageResult<ImportJob>> {
  const qs = new URLSearchParams();
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PageResult<ImportJob>>(`/imports${q ? '?' + q : ''}`);
}

function getPreview(
  jobId: string,
  params?: { page?: number; per_page?: number },
): Promise<PreviewResponse> {
  const qs = new URLSearchParams();
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PreviewResponse>(`/imports/${jobId}${q ? '?' + q : ''}`);
}

function commit(jobId: string): Promise<ImportJob> {
  return apiClient.post<ImportJob>(`/imports/${jobId}/commit`);
}

function rollback(jobId: string): Promise<ImportJob> {
  return apiClient.post<ImportJob>(`/imports/${jobId}/rollback`);
}

function errorsFileUrl(jobId: string, format: 'csv' | 'xlsx' = 'csv'): string {
  return `/api/v1/imports/${jobId}/errors.csv?format=${format}`;
}

function templateFileUrl(importType: ImportType, format: 'csv' | 'xlsx' = 'csv'): string {
  return `/api/v1/imports/template/${importType}?format=${format}`;
}

export const importsApi = {
  upload: uploadFile,
  list,
  getPreview,
  commit,
  rollback,
  errorsFileUrl,
  templateFileUrl,
};
