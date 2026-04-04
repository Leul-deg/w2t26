// AuthContext provides authentication state and operations throughout the app.
//
// Session lifecycle:
//  1. On mount, GET /auth/me verifies the httpOnly cookie with the server.
//  2. The 401 interception handler in apiClient calls forceLogout() for any
//     401 received while authenticated (e.g. session expired mid-use).
//  3. AppShell's useSessionTimeout hook calls forceLogout() after client-side
//     idle timeout as a belt-and-suspenders measure.

import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';
import { apiClient, clearUnauthorizedHandler, setUnauthorizedHandler } from '../api/client';

// ── Types ─────────────────────────────────────────────────────────────────────

export interface AuthUserCore {
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

// AuthUser matches the server's UserWithRoles JSON shape (after json tags were added).
export interface AuthUser {
  user: AuthUserCore;
  roles: string[];
  permissions: string[];
}

export interface LoginRequest {
  username: string;
  password: string;
  captcha_key?: string;
  captcha_answer?: string;
}

type AuthStatus = 'loading' | 'unauthenticated' | 'authenticated';

export interface AuthState {
  status: AuthStatus;
  user: AuthUser | null;
}

interface AuthContextValue {
  auth: AuthState;
  login: (req: LoginRequest) => Promise<void>;
  logout: () => Promise<void>;
  /** Force-clear auth state without calling /auth/logout (used on session expiry). */
  forceLogout: () => void;
  hasPermission: (perm: string) => boolean;
  hasRole: (role: string) => boolean;
  getPrimaryRole: () => string;
}

// ── Context ───────────────────────────────────────────────────────────────────

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [auth, setAuth] = useState<AuthState>({ status: 'loading', user: null });

  // Keep a ref so the 401 handler (a closure) always sees the latest auth state
  // without needing to be re-registered on every render.
  const authRef = useRef(auth);
  authRef.current = auth;

  // On mount: verify session by calling GET /auth/me.
  // The server validates the httpOnly session cookie and returns the current user.
  useEffect(() => {
    apiClient
      .get<AuthUser>('/auth/me')
      .then((user) => setAuth({ status: 'authenticated', user }))
      .catch(() => setAuth({ status: 'unauthenticated', user: null }));
  }, []);

  // Register the global 401 interception handler.
  // Only acts if we are currently authenticated — the initial /auth/me call
  // returning 401 (when not logged in) must not cause a redirect loop.
  useEffect(() => {
    setUnauthorizedHandler(() => {
      if (authRef.current.status === 'authenticated') {
        setAuth({ status: 'unauthenticated', user: null });
      }
    });
    return () => clearUnauthorizedHandler();
  }, []);

  const login = useCallback(async (req: LoginRequest) => {
    const res = await apiClient.post<{ user: AuthUser; captcha_required: boolean }>(
      '/auth/login',
      req,
    );
    setAuth({ status: 'authenticated', user: res.user });
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiClient.post<void>('/auth/logout');
    } catch {
      // Clear local state regardless of server response.
    }
    setAuth({ status: 'unauthenticated', user: null });
  }, []);

  // forceLogout clears state immediately without calling the server.
  // Used by the session timeout hook and 401 interceptor.
  const forceLogout = useCallback(() => {
    setAuth({ status: 'unauthenticated', user: null });
  }, []);

  const hasPermission = useCallback(
    (perm: string) => auth.user?.permissions.includes(perm) ?? false,
    [auth.user],
  );

  const hasRole = useCallback(
    (role: string) => auth.user?.roles.includes(role) ?? false,
    [auth.user],
  );

  const getPrimaryRole = useCallback((): string => {
    if (!auth.user) return 'unknown';
    const { roles } = auth.user;
    if (roles.includes('administrator')) return 'administrator';
    if (roles.includes('content_moderator')) return 'content_moderator';
    if (roles.includes('operations_staff')) return 'operations_staff';
    return roles[0] ?? 'unknown';
  }, [auth.user]);

  return (
    <AuthContext.Provider
      value={{ auth, login, logout, forceLogout, hasPermission, hasRole, getPrimaryRole }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider');
  return ctx;
}
