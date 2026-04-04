// API client for the circulation domain.
// All calls require an active session cookie (credentials: 'include' is set in apiClient).

import { apiClient } from './client';

// ── Types ─────────────────────────────────────────────────────────────────────

export interface CirculationEvent {
  id: string;
  copy_id: string;
  reader_id: string;
  branch_id: string;
  event_type: string;
  due_date?: string;
  returned_at?: string;
  destination_branch_id?: string;
  performed_by?: string;
  workstation_id?: string;
  notes?: string;
  created_at: string;
}

export interface CheckoutRequest {
  copy_id?: string;
  barcode?: string;
  reader_id: string;
  /** Required for administrator users (server branchID = ""); ignored for branch-scoped users. */
  branch_id?: string;
  due_date: string;
  workstation_id?: string;
  notes?: string;
}

export interface ReturnRequest {
  copy_id?: string;
  barcode?: string;
  /** Required for administrator users (server branchID = ""); ignored for branch-scoped users. */
  branch_id?: string;
  workstation_id?: string;
  notes?: string;
}

export interface PageResult<T> {
  items: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

// ── API functions ─────────────────────────────────────────────────────────────

export const circulationApi = {
  /** Check out a copy to a reader. */
  checkout(data: CheckoutRequest) {
    return apiClient.post<CirculationEvent>('/circulation/checkout', data);
  },

  /** Return a copy. */
  return(data: ReturnRequest) {
    return apiClient.post<CirculationEvent>('/circulation/return', data);
  },

  /** List circulation events with optional filters and pagination. */
  list(params?: { page?: number; per_page?: number; event_type?: string }) {
    const qs = new URLSearchParams();
    if (params?.page) qs.set('page', String(params.page));
    if (params?.per_page) qs.set('per_page', String(params.per_page));
    if (params?.event_type) qs.set('event_type', params.event_type);
    const q = qs.toString() ? `?${qs}` : '';
    return apiClient.get<PageResult<CirculationEvent>>(`/circulation${q}`);
  },

  /** List circulation events for a specific copy. */
  listByCopy(copyId: string, params?: { page?: number; per_page?: number }) {
    const qs = new URLSearchParams();
    if (params?.page) qs.set('page', String(params.page));
    if (params?.per_page) qs.set('per_page', String(params.per_page));
    const q = qs.toString() ? `?${qs}` : '';
    return apiClient.get<PageResult<CirculationEvent>>(`/circulation/copy/${copyId}${q}`);
  },

  /** List circulation events for a specific reader. */
  listByReader(readerId: string, params?: { page?: number; per_page?: number }) {
    const qs = new URLSearchParams();
    if (params?.page) qs.set('page', String(params.page));
    if (params?.per_page) qs.set('per_page', String(params.per_page));
    const q = qs.toString() ? `?${qs}` : '';
    return apiClient.get<PageResult<CirculationEvent>>(`/circulation/reader/${readerId}${q}`);
  },

  /** Get the active checkout for a copy. */
  getActiveCheckout(copyId: string) {
    return apiClient.get<CirculationEvent | null>(`/circulation/active/${copyId}`);
  },
};
