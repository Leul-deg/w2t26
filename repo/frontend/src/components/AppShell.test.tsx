// AppShell tests: role-based navigation rendering and session timeout behavior.

import { render, screen, fireEvent, act } from '@testing-library/react';
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import * as AuthContextModule from '../auth/AuthContext';
import type { AuthUser } from '../auth/AuthContext';
import AppShell from './AppShell';

// ── Fixtures ──────────────────────────────────────────────────────────────────

function makeUser(roles: string[], permissions: string[] = []): AuthUser {
  return {
    user: {
      id: 'test-id',
      username: 'testuser',
      email: 'test@lms.local',
      is_active: true,
      failed_attempts: 0,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    },
    roles,
    permissions,
  };
}

const adminUser = makeUser(['administrator'], ['readers:read', 'users:admin', 'reports:read']);
const staffUser = makeUser(['operations_staff'], ['readers:read', 'holdings:read', 'reports:read']);
const moderatorUser = makeUser(['content_moderator'], ['content:moderate', 'feedback:moderate', 'reports:read', 'readers:read']);

// ── Test helpers ──────────────────────────────────────────────────────────────

function mockUseAuth(user: AuthUser, extra?: Partial<ReturnType<typeof AuthContextModule.useAuth>>) {
  vi.spyOn(AuthContextModule, 'useAuth').mockReturnValue({
    auth: { status: 'authenticated', user },
    login: vi.fn(),
    logout: vi.fn().mockResolvedValue(undefined),
    forceLogout: vi.fn(),
    hasPermission: (p: string) => user.permissions.includes(p),
    hasRole: (r: string) => user.roles.includes(r),
    getPrimaryRole: () => {
      if (user.roles.includes('administrator')) return 'administrator';
      if (user.roles.includes('content_moderator')) return 'content_moderator';
      return 'operations_staff';
    },
    ...extra,
  });
}

function renderShell(_user: AuthUser, initialPath = '/dashboard') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route element={<AppShell />}>
          <Route path="/dashboard" element={<div data-testid="page-content">Dashboard</div>} />
          <Route path="/readers" element={<div data-testid="page-content">Readers</div>} />
          <Route path="/users" element={<div data-testid="page-content">Users</div>} />
          <Route path="/moderation" element={<div data-testid="page-content">Moderation</div>} />
        </Route>
        <Route path="/login" element={<div data-testid="login-page">Login</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

// ── Role-based navigation tests ───────────────────────────────────────────────

describe('AppShell — role-based navigation', () => {
  afterEach(() => vi.restoreAllMocks());

  it('shows the sidebar navigation', () => {
    mockUseAuth(adminUser);
    renderShell(adminUser);
    expect(screen.getByRole('navigation', { name: /main navigation/i })).toBeInTheDocument();
  });

  it('administrator sees all navigation items including Users', () => {
    mockUseAuth(adminUser);
    renderShell(adminUser);

    // Admin-only item
    expect(screen.getByRole('link', { name: 'Users' })).toBeInTheDocument();
    // Ops items
    expect(screen.getByRole('link', { name: 'Holdings & Copies' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Circulation' })).toBeInTheDocument();
    // Content items
    expect(screen.getByRole('link', { name: 'Moderation Queue' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Content' })).toBeInTheDocument();
  });

  it('operations_staff does NOT see Users or Moderation Queue', () => {
    mockUseAuth(staffUser);
    renderShell(staffUser);

    expect(screen.queryByRole('link', { name: 'Users' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Moderation Queue' })).not.toBeInTheDocument();

    // But does see ops items
    expect(screen.getByRole('link', { name: 'Holdings & Copies' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Stocktake' })).toBeInTheDocument();
  });

  it('operations_staff sees Readers and Enrollments', () => {
    mockUseAuth(staffUser);
    renderShell(staffUser);

    expect(screen.getByRole('link', { name: 'Readers' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Enrollments' })).toBeInTheDocument();
  });

  it('content_moderator does NOT see Holdings, Circulation, or Stocktake', () => {
    mockUseAuth(moderatorUser);
    renderShell(moderatorUser);

    expect(screen.queryByRole('link', { name: 'Holdings & Copies' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Circulation' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Stocktake' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Users' })).not.toBeInTheDocument();
  });

  it('content_moderator sees Moderation Queue, Content, Feedback, and Appeals', () => {
    mockUseAuth(moderatorUser);
    renderShell(moderatorUser);

    expect(screen.getByRole('link', { name: 'Moderation Queue' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Content' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Feedback' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Appeals' })).toBeInTheDocument();
  });

  it('all roles see Dashboard and Reports', () => {
    for (const user of [adminUser, staffUser, moderatorUser]) {
      mockUseAuth(user);
      const { unmount } = renderShell(user);
      expect(screen.getByRole('link', { name: 'Dashboard' })).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'Reports' })).toBeInTheDocument();
      unmount();
      vi.restoreAllMocks();
    }
  });

  it('displays the current username and role in the sidebar footer', () => {
    mockUseAuth(staffUser);
    renderShell(staffUser);
    // Username appears in both the sidebar and topbar — check at least one exists
    expect(screen.getAllByText('testuser').length).toBeGreaterThan(0);
    expect(screen.getAllByText(/operations staff/i).length).toBeGreaterThan(0);
  });

  it('shows "All branches" for administrator', () => {
    mockUseAuth(adminUser);
    renderShell(adminUser);
    expect(screen.getAllByText(/all branches/i).length).toBeGreaterThan(0);
  });

  it('renders the main content area via Outlet', () => {
    mockUseAuth(adminUser);
    renderShell(adminUser);
    expect(screen.getByTestId('page-content')).toBeInTheDocument();
  });
});

// ── Session timeout tests ─────────────────────────────────────────────────────

describe('AppShell — session timeout', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('does not show the timeout banner before the warn threshold', async () => {
    mockUseAuth(adminUser);
    renderShell(adminUser);

    // 20 minutes — below the 25-minute warn threshold
    await act(async () => { vi.advanceTimersByTime(20 * 60 * 1000); });

    expect(screen.queryByTestId('session-timeout-banner')).not.toBeInTheDocument();
  });

  it('shows the timeout banner after 25 minutes of inactivity', async () => {
    mockUseAuth(adminUser);
    renderShell(adminUser);

    await act(async () => { vi.advanceTimersByTime(25 * 60 * 1000 + 100); });

    expect(screen.getByTestId('session-timeout-banner')).toBeInTheDocument();
    expect(screen.getByText(/session expiring soon/i)).toBeInTheDocument();
  });

  it('hides the timeout banner when the user clicks Stay signed in', async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = vi.fn().mockResolvedValue({
      status: 200,
      ok: true,
      json: async () => adminUser,
    });

    mockUseAuth(adminUser);
    renderShell(adminUser);

    await act(async () => { vi.advanceTimersByTime(25 * 60 * 1000 + 100); });
    expect(screen.getByTestId('session-timeout-banner')).toBeInTheDocument();

    // Click "Stay signed in" — triggers extendSession() which resets the warn timer
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /stay signed in/i }));
      // Allow the fetch promise to resolve
      await Promise.resolve();
    });

    expect(screen.queryByTestId('session-timeout-banner')).not.toBeInTheDocument();

    globalThis.fetch = originalFetch;
  });

  it('calls forceLogout after 30 minutes of inactivity', async () => {
    const forceLogout = vi.fn();
    mockUseAuth(adminUser, { forceLogout });
    renderShell(adminUser);

    await act(async () => { vi.advanceTimersByTime(30 * 60 * 1000 + 100); });

    expect(forceLogout).toHaveBeenCalled();
  });

  it('logout button calls logout and navigates to /login', async () => {
    vi.useRealTimers(); // No fake timers needed for this sub-test
    const logout = vi.fn().mockResolvedValue(undefined);
    mockUseAuth(adminUser, { logout });
    renderShell(adminUser);

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /sign out/i }));
    });

    expect(logout).toHaveBeenCalled();
    // Navigation to /login triggers showing the login-page element in our test router
    expect(screen.getByTestId('login-page')).toBeInTheDocument();
  });
});
