// API client for the content moderation queue.

import { apiClient } from './client';
import type { PageResult } from './readers';
import type { GovernedContent } from './content';

export interface ModerationItem {
  id: string;
  content_id: string;
  assigned_to?: string;
  status: 'pending' | 'in_review' | 'decided';
  decision?: 'approved' | 'rejected';
  decision_reason?: string;
  decided_by?: string;
  decided_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ModerationItemWithContent {
  item: ModerationItem;
  content: GovernedContent;
}

function listQueue(params?: {
  status?: string;
  assigned_to?: string;
  page?: number;
  per_page?: number;
}): Promise<PageResult<ModerationItem>> {
  const qs = new URLSearchParams();
  if (params?.status) qs.set('status', params.status);
  if (params?.assigned_to) qs.set('assigned_to', params.assigned_to);
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PageResult<ModerationItem>>(`/moderation/queue${q ? '?' + q : ''}`);
}

function getItem(id: string): Promise<ModerationItemWithContent> {
  return apiClient.get<ModerationItemWithContent>(`/moderation/items/${id}`);
}

function assignItem(id: string): Promise<ModerationItem> {
  return apiClient.post<ModerationItem>(`/moderation/items/${id}/assign`, {});
}

function decideItem(id: string, decision: 'approved' | 'rejected', reason?: string): Promise<ModerationItem> {
  return apiClient.post<ModerationItem>(`/moderation/items/${id}/decide`, { decision, reason: reason ?? '' });
}

export const moderationApi = {
  listQueue,
  getItem,
  assignItem,
  decideItem,
};
