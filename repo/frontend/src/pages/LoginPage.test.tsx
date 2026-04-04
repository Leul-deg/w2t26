// Tests for LoginPage: form rendering, credential submission, CAPTCHA flow,
// lockout display, and invalid credentials handling.

import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AuthProvider } from '../auth/AuthContext';
import LoginPage from './LoginPage';
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
  permissions: ['readers:read'],
};

function mockFetchSequence(responses: Array<{ status: number; body: unknown }>) {
  let call = 0;
  return vi.fn().mockImplementation(() => {
    const r = responses[call] ?? responses[responses.length - 1];
    call++;
    return Promise.resolve({
      status: r.status,
      ok: r.status >= 200 && r.status < 300,
      json: async () => r.body,
    });
  });
}

function renderLoginPage(initialPath = '/login') {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/dashboard" element={<div data-testid="dashboard">Dashboard</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  );
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('LoginPage', () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it('renders the sign-in form', async () => {
    globalThis.fetch = mockFetchSequence([{ status: 401, body: { error: 'unauthenticated' } }]);

    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    });
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
  });

  it('shows "invalid credentials" message on 422 response', async () => {
    globalThis.fetch = mockFetchSequence([
      { status: 401, body: { error: 'unauthenticated' } }, // /auth/me on mount
      { status: 422, body: { error: 'validation_error', detail: 'invalid credentials' } }, // /auth/login
    ]);

    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'baduser' } });
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'wrongpass' } });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.getByRole('alert').textContent).toMatch(/invalid username or password/i);
  });

  it('shows CAPTCHA challenge on 428 response', async () => {
    globalThis.fetch = mockFetchSequence([
      { status: 401, body: { error: 'unauthenticated' } }, // /auth/me
      {
        status: 428,
        body: {
          error: 'captcha_required',
          challenge_key: 'abc123',
          challenge: 'What is 3 + 5?',
        },
      }, // /auth/login
    ]);

    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'user' } });
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'wrongpass' } });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByText(/security check required/i)).toBeInTheDocument();
    });
    expect(screen.getByText('What is 3 + 5?')).toBeInTheDocument();
    expect(screen.getByLabelText(/answer/i)).toBeInTheDocument();
  });

  it('shows account locked message on 423 response', async () => {
    globalThis.fetch = mockFetchSequence([
      { status: 401, body: { error: 'unauthenticated' } }, // /auth/me
      { status: 423, body: { error: 'account_locked', retry_after_seconds: 900 } }, // /auth/login
    ]);

    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'locked' } });
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'anypass' } });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert').textContent).toMatch(/account locked/i);
    });
  });

  it('redirects to /dashboard on successful login', async () => {
    globalThis.fetch = mockFetchSequence([
      { status: 401, body: { error: 'unauthenticated' } }, // /auth/me
      { status: 200, body: { user: adminUser, captcha_required: false } }, // /auth/login
    ]);

    renderLoginPage();

    await waitFor(() => {
      expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/username/i), { target: { value: 'admin' } });
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'Admin1234!' } });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByTestId('dashboard')).toBeInTheDocument();
    });
  });

  it('shows session expired message when ?reason=expired is in the URL', async () => {
    globalThis.fetch = mockFetchSequence([{ status: 401, body: { error: 'unauthenticated' } }]);

    renderLoginPage('/login?reason=expired');

    await waitFor(() => {
      expect(screen.getByRole('status')).toBeInTheDocument();
    });
    expect(screen.getByRole('status').textContent).toMatch(/session has expired/i);
  });
});
