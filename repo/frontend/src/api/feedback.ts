// API client for reader feedback.

import { apiClient } from './client';
import type { PageResult } from './readers';

export interface FeedbackTag {
  id: string;
  name: string;
  is_active: boolean;
}

export interface Feedback {
  id: string;
  branch_id: string;
  reader_id: string;
  target_type: 'holding' | 'program';
  target_id: string;
  rating?: number;
  comment?: string;
  tags: string[];
  status: 'pending' | 'approved' | 'rejected' | 'flagged';
  moderated_by?: string;
  moderated_at?: string;
  submitted_at: string;
}

function listFeedback(params?: {
  status?: string;
  target_type?: string;
  target_id?: string;
  page?: number;
  per_page?: number;
}): Promise<PageResult<Feedback>> {
  const qs = new URLSearchParams();
  if (params?.status) qs.set('status', params.status);
  if (params?.target_type) qs.set('target_type', params.target_type);
  if (params?.target_id) qs.set('target_id', params.target_id);
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PageResult<Feedback>>(`/feedback${q ? '?' + q : ''}`);
}

function getFeedback(id: string): Promise<Feedback> {
  return apiClient.get<Feedback>(`/feedback/${id}`);
}

function listTags(): Promise<FeedbackTag[]> {
  return apiClient.get<FeedbackTag[]>('/feedback/tags');
}

function submitFeedback(data: {
  reader_id: string;
  target_type: 'holding' | 'program';
  target_id: string;
  rating?: number;
  comment?: string;
  tags?: string[];
}): Promise<Feedback> {
  return apiClient.post<Feedback>('/feedback', { ...data, tags: data.tags ?? [] });
}

function moderateFeedback(id: string, status: 'approved' | 'rejected' | 'flagged'): Promise<Feedback> {
  return apiClient.post<Feedback>(`/feedback/${id}/moderate`, { status });
}

export const feedbackApi = {
  list: listFeedback,
  get: getFeedback,
  listTags,
  submit: submitFeedback,
  moderate: moderateFeedback,
};
