// API client for governed content.

import { apiClient } from './client';
import type { PageResult } from './readers';

export type ContentStatus = 'draft' | 'pending_review' | 'approved' | 'rejected' | 'published' | 'archived';
export type ContentType = 'announcement' | 'document' | 'digital_resource' | 'policy';

export interface GovernedContent {
  id: string;
  branch_id: string;
  title: string;
  content_type: ContentType;
  body?: string;
  file_name?: string;
  status: ContentStatus;
  submitted_by: string;
  submitted_at: string;
  published_at?: string;
  archived_at?: string;
  rejection_reason?: string;
  created_at: string;
  updated_at: string;
}

function listContent(params?: {
  status?: string;
  content_type?: string;
  page?: number;
  per_page?: number;
}): Promise<PageResult<GovernedContent>> {
  const qs = new URLSearchParams();
  if (params?.status) qs.set('status', params.status);
  if (params?.content_type) qs.set('content_type', params.content_type);
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PageResult<GovernedContent>>(`/content${q ? '?' + q : ''}`);
}

function getContent(id: string): Promise<GovernedContent> {
  return apiClient.get<GovernedContent>(`/content/${id}`);
}

function createContent(data: {
  title: string;
  content_type: ContentType;
  body?: string;
  file_name?: string;
}): Promise<GovernedContent> {
  return apiClient.post<GovernedContent>('/content', data);
}

function updateContent(id: string, data: Partial<{
  title: string;
  body: string;
  file_name: string;
}>): Promise<GovernedContent> {
  return apiClient.patch<GovernedContent>(`/content/${id}`, data);
}

function submitContent(id: string): Promise<GovernedContent> {
  return apiClient.post<GovernedContent>(`/content/${id}/submit`, {});
}

function retractContent(id: string): Promise<GovernedContent> {
  return apiClient.post<GovernedContent>(`/content/${id}/retract`, {});
}

function publishContent(id: string): Promise<GovernedContent> {
  return apiClient.post<GovernedContent>(`/content/${id}/publish`, {});
}

function archiveContent(id: string): Promise<GovernedContent> {
  return apiClient.post<GovernedContent>(`/content/${id}/archive`, {});
}

export const contentApi = {
  list: listContent,
  get: getContent,
  create: createContent,
  update: updateContent,
  submit: submitContent,
  retract: retractContent,
  publish: publishContent,
  archive: archiveContent,
};
