// API client for the stocktake domain.
// All calls require an active session cookie (credentials: 'include' is set in apiClient).

import { apiClient } from './client';

// ── Types ─────────────────────────────────────────────────────────────────────

export interface StocktakeSession {
  id: string;
  branch_id: string;
  name: string;
  status: string;
  started_by: string;
  notes?: string;
  started_at: string;
  closed_at?: string;
  created_at: string;
}

export interface StocktakeFinding {
  id: string;
  session_id: string;
  copy_id: string;
  barcode?: string;
  finding_type: string;
  notes?: string;
  created_at: string;
}

export interface StocktakeVariance {
  copy_id: string;
  barcode: string;
  expected_status: string;
  found_status: string;
  finding_type: string;
}

export interface PageResult<T> {
  items: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

// ── API functions ─────────────────────────────────────────────────────────────

export const stocktakeApi = {
  /** List stocktake sessions with optional filters and pagination. */
  list(params?: { page?: number; per_page?: number }) {
    const qs = new URLSearchParams();
    if (params?.page) qs.set('page', String(params.page));
    if (params?.per_page) qs.set('per_page', String(params.per_page));
    const q = qs.toString() ? `?${qs}` : '';
    return apiClient.get<PageResult<StocktakeSession>>(`/stocktake${q}`);
  },

  /** Get a single stocktake session by ID. */
  get(id: string) {
    return apiClient.get<StocktakeSession>(`/stocktake/${id}`);
  },

  /** Create a new stocktake session. */
  create(data: { name: string; notes?: string }) {
    return apiClient.post<StocktakeSession>('/stocktake', data);
  },

  /** Close or cancel a stocktake session. */
  close(id: string, status: string) {
    return apiClient.patch<StocktakeSession>(`/stocktake/${id}/status`, { status });
  },

  /** List findings for a stocktake session. */
  listFindings(id: string, params?: { page?: number; per_page?: number }) {
    const qs = new URLSearchParams();
    if (params?.page) qs.set('page', String(params.page));
    if (params?.per_page) qs.set('per_page', String(params.per_page));
    const q = qs.toString() ? `?${qs}` : '';
    return apiClient.get<PageResult<StocktakeFinding>>(`/stocktake/${id}/findings${q}`);
  },

  /** Record a barcode scan in the session. */
  recordScan(id: string, barcode: string, findingType?: string, notes?: string) {
    return apiClient.post<StocktakeFinding>(`/stocktake/${id}/scan`, {
      barcode,
      ...(findingType ? { finding_type: findingType } : {}),
      ...(notes ? { notes } : {}),
    });
  },

  /** Get variances for a stocktake session. */
  getVariances(id: string) {
    return apiClient.get<StocktakeVariance[]>(`/stocktake/${id}/variances`);
  },
};
