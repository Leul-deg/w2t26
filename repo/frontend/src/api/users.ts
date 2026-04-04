import { apiClient } from './client';

export interface User {
  id: string;
  username: string;
  email: string;
  is_active: boolean;
  failed_attempts: number;
  locked_until?: string;
  last_login_at?: string;
  created_at: string;
  updated_at: string;
}

export interface Role {
  id: string;
  name: string;
  description: string;
}

export interface UserDetail {
  user: User;
  roles: Role[];
  branch_ids: string[];
}

export interface UserListItem extends User {
  roles: Role[];
  branch_ids: string[];
}

export interface UserListResponse {
  items: UserListItem[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

export interface CreateUserRequest {
  username: string;
  email: string;
  password: string;
  is_active?: boolean;
}

export interface UpdateUserRequest {
  email?: string;
  is_active?: boolean;
}

export const usersApi = {
  list(params?: { page?: number; per_page?: number }): Promise<UserListResponse> {
    const p = new URLSearchParams();
    if (params?.page) p.set('page', String(params.page));
    if (params?.per_page) p.set('per_page', String(params.per_page));
    const qs = p.toString();
    return apiClient.get<UserListResponse>(`/users${qs ? `?${qs}` : ''}`);
  },

  create(req: CreateUserRequest): Promise<User> {
    return apiClient.post<User>('/users', req);
  },

  get(id: string): Promise<UserDetail> {
    return apiClient.get<UserDetail>(`/users/${id}`);
  },

  update(id: string, req: UpdateUserRequest): Promise<User> {
    return apiClient.patch<User>(`/users/${id}`, req);
  },

  assignRole(userID: string, roleID: string): Promise<{ role_id: string }> {
    return apiClient.post<{ role_id: string }>(`/users/${userID}/roles`, { role_id: roleID });
  },

  revokeRole(userID: string, roleID: string): Promise<void> {
    return apiClient.delete<void>(`/users/${userID}/roles/${roleID}`);
  },

  assignBranch(userID: string, branchID: string): Promise<{ branch_id: string }> {
    return apiClient.post<{ branch_id: string }>(`/users/${userID}/branches`, { branch_id: branchID });
  },

  revokeBranch(userID: string, branchID: string): Promise<void> {
    return apiClient.delete<void>(`/users/${userID}/branches/${branchID}`);
  },
};
