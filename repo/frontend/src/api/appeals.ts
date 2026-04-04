// API client for appeals and arbitration.

import { apiClient } from './client';
import type { PageResult } from './readers';

export type AppealType = 'enrollment_denial' | 'account_suspension' | 'feedback_rejection' | 'blacklist_removal' | 'other';
export type AppealStatus = 'submitted' | 'under_review' | 'resolved' | 'dismissed';

export interface Appeal {
  id: string;
  branch_id: string;
  reader_id: string;
  appeal_type: AppealType;
  target_type?: string;
  target_id?: string;
  reason: string;
  status: AppealStatus;
  submitted_at: string;
  updated_at: string;
}

export interface AppealArbitration {
  id: string;
  appeal_id: string;
  arbitrator_id: string;
  decision: 'upheld' | 'dismissed' | 'partial';
  decision_notes: string;
  before_state?: unknown;
  after_state?: unknown;
  decided_at: string;
}

export interface AppealDetail {
  appeal: Appeal;
  arbitration: AppealArbitration | null;
}

function listAppeals(params?: {
  status?: string;
  appeal_type?: string;
  reader_id?: string;
  page?: number;
  per_page?: number;
}): Promise<PageResult<Appeal>> {
  const qs = new URLSearchParams();
  if (params?.status) qs.set('status', params.status);
  if (params?.appeal_type) qs.set('appeal_type', params.appeal_type);
  if (params?.reader_id) qs.set('reader_id', params.reader_id);
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PageResult<Appeal>>(`/appeals${q ? '?' + q : ''}`);
}

function getAppeal(id: string): Promise<AppealDetail> {
  return apiClient.get<AppealDetail>(`/appeals/${id}`);
}

function submitAppeal(data: {
  reader_id: string;
  appeal_type: AppealType;
  reason: string;
  target_type?: string;
  target_id?: string;
}): Promise<Appeal> {
  return apiClient.post<Appeal>('/appeals', data);
}

function reviewAppeal(id: string): Promise<Appeal> {
  return apiClient.post<Appeal>(`/appeals/${id}/review`, {});
}

function arbitrateAppeal(id: string, data: {
  decision: 'upheld' | 'dismissed' | 'partial';
  decision_notes: string;
  before_state?: unknown;
  after_state?: unknown;
}): Promise<AppealDetail> {
  return apiClient.post<AppealDetail>(`/appeals/${id}/arbitrate`, data);
}

export const appealsApi = {
  list: listAppeals,
  get: getAppeal,
  submit: submitAppeal,
  review: reviewAppeal,
  arbitrate: arbitrateAppeal,
};
