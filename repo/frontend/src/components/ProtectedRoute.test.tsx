// Tests for ProtectedRoute: redirection behavior based on auth state.

import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AuthProvider } from '../auth/AuthContext';
import ProtectedRoute from './ProtectedRoute';
import type { AuthUser } from '../auth/AuthContext';

// ── Fixtures ──────────────────────────────────────────────────────────────────

const adminUser: AuthUser = {
  user: {
    id: 'aaaaaaaa-0000-0000-0000-000000000001',
    username: 'admin',
    email: 'admin@lms.local',
    is_active: true,
    failed_attempts: 0,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  roles: ['administrator'],
  permissions: ['readers:read', 'users:admin'],
};

const staffUser: AuthUser = {
  user: {
    id: 'bbbb-0002',
    username: 'staff',
    email: 'staff@lms.local',
    is_active: true,
    failed_attempts: 0,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  roles: ['operations_staff'],
  permissions: ['readers:read'],
};

function mockFetch(status: number, body: unknown) {
  return vi.fn().mockResolvedValue({
    status,
    ok: status >= 200 && status < 300,
    json: async () => body,
  });
}

function renderWithRoute(initialPath: string, fetchMock: ReturnType<typeof vi.fn>) {
  globalThis.fetch = fetchMock;
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/login" element={<div data-testid="login-page">Login</div>} />
          <Route path="/unauthorized" element={<div data-testid="unauthorized-page">Forbidden</div>} />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <div data-testid="dashboard">Dashboard</div>
              </ProtectedRoute>
            }
          />
          <Route
            path="/admin-only"
            element={
              <ProtectedRoute requireRole="administrator">
                <div data-testid="admin-only">Admin Only</div>
              </ProtectedRoute>
            }
          />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  );
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('ProtectedRoute', () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it('redirects to /login when unauthenticated', async () => {
    renderWithRoute('/dashboard', mockFetch(401, { error: 'unauthenticated' }));

    await waitFor(() => {
      expect(screen.queryByTestId('login-page')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('dashboard')).not.toBeInTheDocument();
  });

  it('renders children when authenticated', async () => {
    renderWithRoute('/dashboard', mockFetch(200, adminUser));

    await waitFor(() => {
      expect(screen.getByTestId('dashboard')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('login-page')).not.toBeInTheDocument();
  });

  it('redirects to /unauthorized when authenticated but missing required role', async () => {
    renderWithRoute('/admin-only', mockFetch(200, staffUser));

    await waitFor(() => {
      expect(screen.queryByTestId('unauthorized-page')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('admin-only')).not.toBeInTheDocument();
  });

  it('renders admin-only content when user has the required role', async () => {
    renderWithRoute('/admin-only', mockFetch(200, adminUser));

    await waitFor(() => {
      expect(screen.getByTestId('admin-only')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('unauthorized-page')).not.toBeInTheDocument();
  });

  it('shows nothing (null) while session check is in progress', () => {
    // Fetch never resolves — simulates in-flight /auth/me.
    globalThis.fetch = vi.fn().mockReturnValue(new Promise(() => {}));

    render(
      <AuthProvider>
        <MemoryRouter initialEntries={['/dashboard']}>
          <Routes>
            <Route
              path="/dashboard"
              element={
                <ProtectedRoute>
                  <div data-testid="dashboard">Dashboard</div>
                </ProtectedRoute>
              }
            />
          </Routes>
        </MemoryRouter>
      </AuthProvider>,
    );

    // While loading, neither the login redirect nor the dashboard should be visible.
    expect(screen.queryByTestId('dashboard')).not.toBeInTheDocument();
    expect(screen.queryByTestId('login-page')).not.toBeInTheDocument();
  });
});
