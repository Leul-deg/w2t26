// API client for the programs and enrollment domain.

import { apiClient } from './client';
import type { PageResult } from './readers';

// ── Types ─────────────────────────────────────────────────────────────────────

export type ProgramStatus = 'draft' | 'published' | 'cancelled' | 'completed';
export type EnrollmentChannel = 'any' | 'staff_only' | 'self_service';

export interface Program {
  id: string;
  branch_id: string;
  title: string;
  description?: string;
  category?: string;
  venue_type?: string;
  venue_name?: string;
  capacity: number;
  enrollment_opens_at?: string;
  enrollment_closes_at?: string;
  starts_at: string;
  ends_at: string;
  status: ProgramStatus;
  enrollment_channel: EnrollmentChannel;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface ProgramPrerequisite {
  id: string;
  program_id: string;
  required_program_id: string;
  description?: string;
  created_at: string;
}

export interface EnrollmentRule {
  id: string;
  program_id: string;
  rule_type: 'whitelist' | 'blacklist';
  match_field: string;
  match_value: string;
  reason?: string;
  created_at: string;
}

export interface ProgramDetail extends Program {
  prerequisites: ProgramPrerequisite[];
  enrollment_rules: EnrollmentRule[];
}

export type EnrollmentStatus =
  | 'pending'
  | 'confirmed'
  | 'waitlisted'
  | 'cancelled'
  | 'completed'
  | 'no_show';

export interface Enrollment {
  id: string;
  program_id: string;
  reader_id: string;
  branch_id: string;
  status: EnrollmentStatus;
  enrollment_channel?: string;
  waitlist_position?: number;
  enrolled_at: string;
  updated_at: string;
  enrolled_by?: string;
  remaining_seats?: number;
}

export interface EnrollmentHistory {
  id: string;
  enrollment_id: string;
  previous_status: string;
  new_status: string;
  changed_by?: string;
  reason?: string;
  changed_at: string;
}

export interface EligibilityError {
  error: string; // closed_window | not_published | reader_ineligible | blacklisted | not_whitelisted | prerequisite_not_met | conflict
  detail: string;
}

// ── API calls ─────────────────────────────────────────────────────────────────

function listPrograms(params?: {
  status?: string;
  category?: string;
  search?: string;
  page?: number;
  per_page?: number;
}): Promise<PageResult<Program>> {
  const qs = new URLSearchParams();
  if (params?.status) qs.set('status', params.status);
  if (params?.category) qs.set('category', params.category);
  if (params?.search) qs.set('search', params.search);
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PageResult<Program>>(`/programs${q ? '?' + q : ''}`);
}

function getProgram(id: string): Promise<ProgramDetail> {
  return apiClient.get<ProgramDetail>(`/programs/${id}`);
}

function createProgram(data: {
  branch_id?: string;
  title: string;
  description?: string;
  category?: string;
  venue_type?: string;
  venue_name?: string;
  capacity: number;
  enrollment_opens_at?: string;
  enrollment_closes_at?: string;
  starts_at: string;
  ends_at: string;
  enrollment_channel?: string;
}): Promise<Program> {
  return apiClient.post<Program>('/programs', data);
}

function updateProgram(id: string, data: Partial<{
  title: string;
  description: string;
  category: string;
  venue_type: string;
  venue_name: string;
  capacity: number;
  enrollment_opens_at: string;
  enrollment_closes_at: string;
  starts_at: string;
  ends_at: string;
  enrollment_channel: string;
}>): Promise<Program> {
  return apiClient.patch<Program>(`/programs/${id}`, data);
}

function updateProgramStatus(id: string, status: ProgramStatus): Promise<{ status: string }> {
  return apiClient.patch<{ status: string }>(`/programs/${id}/status`, { status });
}

function getSeats(programId: string): Promise<{ remaining_seats: number }> {
  return apiClient.get<{ remaining_seats: number }>(`/programs/${programId}/seats`);
}

// Enrollment operations
async function enroll(programId: string, readerId: string, channel?: string): Promise<Enrollment> {
  return apiClient.post<Enrollment>(`/programs/${programId}/enroll`, {
    reader_id: readerId,
    enrollment_channel: channel ?? 'any',
  });
}

function listEnrollmentsByProgram(programId: string, params?: { page?: number; per_page?: number }): Promise<PageResult<Enrollment>> {
  const qs = new URLSearchParams();
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PageResult<Enrollment>>(`/programs/${programId}/enrollments${q ? '?' + q : ''}`);
}

function listEnrollmentsByReader(readerId: string, params?: { page?: number; per_page?: number }): Promise<PageResult<Enrollment>> {
  const qs = new URLSearchParams();
  if (params?.page) qs.set('page', String(params.page));
  if (params?.per_page) qs.set('per_page', String(params.per_page));
  const q = qs.toString();
  return apiClient.get<PageResult<Enrollment>>(`/readers/${readerId}/enrollments${q ? '?' + q : ''}`);
}

function dropEnrollment(enrollmentId: string, readerId: string, reason?: string): Promise<void> {
  return apiClient.post<void>(`/enrollments/${enrollmentId}/drop`, {
    reader_id: readerId,
    reason: reason ?? '',
  });
}

function getEnrollmentHistory(enrollmentId: string): Promise<EnrollmentHistory[]> {
  return apiClient.get<EnrollmentHistory[]>(`/enrollments/${enrollmentId}/history`);
}

// Prerequisite management
function addPrerequisite(programId: string, requiredProgramId: string, description?: string): Promise<ProgramPrerequisite> {
  return apiClient.post<ProgramPrerequisite>(`/programs/${programId}/prerequisites`, {
    required_program_id: requiredProgramId,
    description,
  });
}

function removePrerequisite(programId: string, requiredProgramId: string): Promise<void> {
  return apiClient.delete<void>(`/programs/${programId}/prerequisites/${requiredProgramId}`);
}

// Rule management
function addRule(programId: string, rule: {
  rule_type: 'whitelist' | 'blacklist';
  match_field: string;
  match_value: string;
  reason?: string;
}): Promise<EnrollmentRule> {
  return apiClient.post<EnrollmentRule>(`/programs/${programId}/rules`, rule);
}

function removeRule(programId: string, ruleId: string): Promise<void> {
  return apiClient.delete<void>(`/programs/${programId}/rules/${ruleId}`);
}

export const programsApi = {
  list: listPrograms,
  get: getProgram,
  create: createProgram,
  update: updateProgram,
  updateStatus: updateProgramStatus,
  getSeats,
  enroll,
  listEnrollmentsByProgram,
  listEnrollmentsByReader,
  dropEnrollment,
  getEnrollmentHistory,
  addPrerequisite,
  removePrerequisite,
  addRule,
  removeRule,
};
