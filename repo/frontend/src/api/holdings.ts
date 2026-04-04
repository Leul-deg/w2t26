// API client for the holdings domain.
// All calls require an active session cookie (credentials: 'include' is set in apiClient).

import { apiClient } from './client';

// ── Types ─────────────────────────────────────────────────────────────────────

export interface Holding {
  id: string;
  branch_id: string;
  title: string;
  author?: string;
  isbn?: string;
  publisher?: string;
  publication_year?: number;
  category?: string;
  subcategory?: string;
  language: string;
  description?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface Copy {
  id: string;
  holding_id: string;
  branch_id: string;
  barcode: string;
  status_code: string;
  condition: string;
  shelf_location?: string;
  acquired_at?: string;
  price_paid?: number;
  notes?: string;
  created_at: string;
  updated_at: string;
}

export interface CopyStatus {
  code: string;
  description: string;
  is_borrowable: boolean;
}

export interface PageResult<T> {
  items: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

// ── API functions ─────────────────────────────────────────────────────────────

export const holdingsApi = {
  /** List holdings with optional filters and pagination. */
  list(params?: { search?: string; category?: string; isbn?: string; active?: boolean; page?: number; per_page?: number }) {
    const qs = new URLSearchParams();
    if (params?.search) qs.set('search', params.search);
    if (params?.category) qs.set('category', params.category);
    if (params?.isbn) qs.set('isbn', params.isbn);
    if (params?.active !== undefined) qs.set('active', String(params.active));
    if (params?.page) qs.set('page', String(params.page));
    if (params?.per_page) qs.set('per_page', String(params.per_page));
    const q = qs.toString() ? `?${qs}` : '';
    return apiClient.get<PageResult<Holding>>(`/holdings${q}`);
  },

  /** Get a single holding by ID. */
  get(id: string) {
    return apiClient.get<Holding>(`/holdings/${id}`);
  },

  /** Create a new holding. */
  create(data: Partial<Holding>) {
    return apiClient.post<Holding>('/holdings', data);
  },

  /** Update a holding's fields. */
  update(id: string, data: Partial<Holding>) {
    return apiClient.patch<Holding>(`/holdings/${id}`, data);
  },

  /** Deactivate (soft-delete) a holding. */
  deactivate(id: string) {
    return apiClient.delete<void>(`/holdings/${id}`);
  },

  /** List copies for a holding. */
  listCopies(holdingId: string, params?: { page?: number; per_page?: number }) {
    const qs = new URLSearchParams();
    if (params?.page) qs.set('page', String(params.page));
    if (params?.per_page) qs.set('per_page', String(params.per_page));
    const q = qs.toString() ? `?${qs}` : '';
    return apiClient.get<PageResult<Copy>>(`/holdings/${holdingId}/copies${q}`);
  },

  /** Add a copy to a holding. */
  addCopy(holdingId: string, data: Partial<Copy>) {
    return apiClient.post<Copy>(`/holdings/${holdingId}/copies`, data);
  },

  /** Look up a copy by barcode. */
  getCopyByBarcode(barcode: string) {
    return apiClient.get<Copy>(`/copies/lookup?barcode=${encodeURIComponent(barcode)}`);
  },

  /** Update the status of a copy. */
  updateCopyStatus(copyId: string, statusCode: string) {
    return apiClient.patch<Copy>(`/copies/${copyId}/status`, { status_code: statusCode });
  },

  /** List all copy status lookup values. */
  listCopyStatuses() {
    return apiClient.get<CopyStatus[]>('/copies/statuses');
  },
};
