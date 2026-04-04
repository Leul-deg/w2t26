// API client for the readers domain.
// All calls require an active session cookie (credentials: 'include' is set in apiClient).

import { apiClient } from './client';

// ── Types ─────────────────────────────────────────────────────────────────────

export interface Reader {
  id: string;
  branch_id: string;
  reader_number: string;
  status_code: string;
  first_name: string;
  last_name: string;
  preferred_name?: string;
  notes?: string;
  registered_at: string;
  created_at: string;
  updated_at: string;
  created_by?: string;
  sensitive_fields: SensitiveFields;
}

export interface SensitiveFields {
  national_id?: string;
  contact_email?: string;
  contact_phone?: string;
  date_of_birth?: string;
}

export interface ReaderStatus {
  code: string;
  description: string;
  allows_borrowing: boolean;
  allows_enrollment: boolean;
}

export interface LoanHistoryItem {
  event_id: string;
  copy_id: string;
  barcode: string;
  title: string;
  author?: string;
  event_type: string;
  due_date?: string;
  returned_at?: string;
  created_at: string;
}

export interface PageResult<T> {
  items: T[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

export interface CreateReaderRequest {
  branch_id?: string;
  reader_number?: string;
  first_name: string;
  last_name: string;
  preferred_name?: string;
  notes?: string;
  national_id?: string;
  contact_email?: string;
  contact_phone?: string;
  date_of_birth?: string;
}

export interface UpdateReaderRequest {
  first_name: string;
  last_name: string;
  preferred_name?: string;
  notes?: string;
  national_id?: string;
  contact_email?: string;
  contact_phone?: string;
  date_of_birth?: string;
}

// ── API functions ─────────────────────────────────────────────────────────────

export const readersApi = {
  /** List readers with optional search/status filter and pagination. */
  list(params?: { search?: string; status?: string; page?: number; per_page?: number }) {
    const qs = new URLSearchParams();
    if (params?.search) qs.set('search', params.search);
    if (params?.status) qs.set('status', params.status);
    if (params?.page) qs.set('page', String(params.page));
    if (params?.per_page) qs.set('per_page', String(params.per_page));
    const q = qs.toString() ? `?${qs}` : '';
    return apiClient.get<PageResult<Reader>>(`/readers${q}`);
  },

  /** Get a single reader by ID. Sensitive fields are masked. */
  get(id: string) {
    return apiClient.get<Reader>(`/readers/${id}`);
  },

  /** Create a new reader. */
  create(data: CreateReaderRequest) {
    return apiClient.post<Reader>('/readers', data);
  },

  /** Update a reader's profile fields. */
  update(id: string, data: UpdateReaderRequest) {
    return apiClient.patch<Reader>(`/readers/${id}`, data);
  },

  /** Change a reader's status (active, frozen, blacklisted, pending_verification). */
  updateStatus(id: string, statusCode: string) {
    return apiClient.patch<{ status_code: string }>(`/readers/${id}/status`, { status_code: statusCode });
  },

  /** Reveal decrypted sensitive fields (step-up must have been completed first). */
  reveal(id: string) {
    return apiClient.post<SensitiveFields>(`/readers/${id}/reveal`, {});
  },

  /** Get paginated loan history for a reader. */
  getLoanHistory(id: string, page?: number) {
    const q = page ? `?page=${page}` : '';
    return apiClient.get<PageResult<LoanHistoryItem>>(`/readers/${id}/history${q}`);
  },

  /** Get currently checked-out items for a reader. */
  getCurrentHoldings(id: string) {
    return apiClient.get<LoanHistoryItem[]>(`/readers/${id}/holdings`);
  },

  /** List all reader status lookup values. */
  listStatuses() {
    return apiClient.get<ReaderStatus[]>('/readers/statuses');
  },
};
