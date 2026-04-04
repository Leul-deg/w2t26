// App integration tests: routing smoke tests for the Phase 5 shell.
//
// These tests use MemoryRouter to control the route without a real browser.
// Fetch is mocked to simulate the server's /auth/me response.

import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';

// ── Helpers ───────────────────────────────────────────────────────────────────

function mockFetch(status: number, body: unknown) {
  return vi.fn().mockResolvedValue({
    status,
    ok: status >= 200 && status < 300,
    json: async () => body,
  });
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('App — Phase 5 shell', () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it('renders the login page at /login when unauthenticated', async () => {
    globalThis.fetch = mockFetch(401, { error: 'unauthenticated' });

    // Use dynamic import to isolate React Router's BrowserRouter to this test.
    const { default: App } = await import('./App');
    render(<App />);

    // The login page should appear after the session check resolves.
    await waitFor(() => {
      expect(
        screen.getByText(/Library Management System/i),
      ).toBeInTheDocument();
    });
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
  });

  it('renders the dashboard for an authenticated administrator', async () => {
    globalThis.fetch = mockFetch(200, {
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
    });

    const { default: App } = await import('./App');
    render(<App />);

    await waitFor(() => {
      expect(screen.getByText(/Welcome, admin/i)).toBeInTheDocument();
    });
  });

  it('shows the sidebar navigation when authenticated', async () => {
    globalThis.fetch = mockFetch(200, {
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
      permissions: ['readers:read'],
    });

    const { default: App } = await import('./App');
    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole('navigation', { name: /main navigation/i })).toBeInTheDocument();
    });
  });
});
