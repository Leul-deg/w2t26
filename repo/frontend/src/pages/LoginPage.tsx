// LoginPage handles username/password login with CAPTCHA and lockout UX.
//
// Backend error codes handled here:
//   422 (validation_error)        — generic "invalid credentials"
//   428 (captcha_required)        — show challenge question + answer input
//   423 (account_locked)          — show lockout message with remaining time
//   other                         — generic error message
//
// On success, redirects to the `next` query param or /dashboard.

import { FormEvent, useState } from 'react';
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { HttpError } from '../api/client';

interface CaptchaState {
  key: string;
  question: string;
}

type FormError =
  | { kind: 'invalid_credentials' }
  | { kind: 'captcha_required'; captcha: CaptchaState }
  | { kind: 'account_locked'; seconds: number }
  | { kind: 'network_error' }
  | { kind: 'generic'; message: string };

const S = {
  page: {
    minHeight: '100vh',
    background: '#f3f4f6',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontFamily: 'system-ui, -apple-system, sans-serif',
    padding: '1rem',
  } as React.CSSProperties,

  card: {
    background: '#ffffff',
    border: '1px solid #e5e7eb',
    borderRadius: '6px',
    padding: '2rem',
    width: '100%',
    maxWidth: '380px',
    boxShadow: '0 1px 3px rgba(0,0,0,0.08)',
  } as React.CSSProperties,

  title: {
    fontSize: '1.125rem',
    fontWeight: 700,
    color: '#1a1a2e',
    marginBottom: '0.25rem',
  } as React.CSSProperties,

  subtitle: {
    fontSize: '0.8125rem',
    color: '#6b7280',
    marginBottom: '1.5rem',
  } as React.CSSProperties,

  label: {
    display: 'block',
    fontSize: '0.8125rem',
    fontWeight: 500,
    color: '#374151',
    marginBottom: '0.375rem',
  } as React.CSSProperties,

  input: {
    display: 'block',
    width: '100%',
    padding: '0.5rem 0.75rem',
    border: '1px solid #d1d5db',
    borderRadius: '4px',
    fontSize: '0.875rem',
    marginBottom: '1rem',
    outline: 'none',
    boxSizing: 'border-box' as const,
    color: '#1a1a2e',
    background: '#fff',
  } as React.CSSProperties,

  submitBtn: {
    display: 'block',
    width: '100%',
    padding: '0.625rem',
    background: '#2563eb',
    color: '#fff',
    border: 'none',
    borderRadius: '4px',
    fontSize: '0.875rem',
    fontWeight: 600,
    cursor: 'pointer',
    marginTop: '0.5rem',
  } as React.CSSProperties,

  errorBox: {
    background: '#fef2f2',
    border: '1px solid #fca5a5',
    borderRadius: '4px',
    padding: '0.75rem 1rem',
    marginBottom: '1rem',
    fontSize: '0.8125rem',
    color: '#dc2626',
  } as React.CSSProperties,

  warnBox: {
    background: '#fef3c7',
    border: '1px solid #f59e0b',
    borderRadius: '4px',
    padding: '0.75rem 1rem',
    marginBottom: '1rem',
    fontSize: '0.8125rem',
    color: '#92400e',
  } as React.CSSProperties,

  captchaBox: {
    background: '#eff6ff',
    border: '1px solid #bfdbfe',
    borderRadius: '4px',
    padding: '0.75rem 1rem',
    marginBottom: '1rem',
    fontSize: '0.8125rem',
    color: '#1e40af',
  } as React.CSSProperties,
};

function formatSeconds(s: number): string {
  const m = Math.floor(s / 60);
  const sec = s % 60;
  if (m > 0) return `${m}m ${sec}s`;
  return `${sec}s`;
}

export default function LoginPage() {
  const { login, auth } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [captchaAnswer, setCaptchaAnswer] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<FormError | null>(null);

  // Current active CAPTCHA challenge (if one has been issued).
  const [captcha, setCaptcha] = useState<CaptchaState | null>(null);

  // If already authenticated, redirect immediately using Navigate (safe during render).
  if (auth.status === 'authenticated') {
    const next = searchParams.get('next') ?? '/dashboard';
    return <Navigate to={next} replace />;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);

    try {
      await login({
        username,
        password,
        captcha_key: captcha?.key,
        captcha_answer: captcha ? captchaAnswer : undefined,
      });

      // Success — navigate to intended destination.
      const next = searchParams.get('next') ?? '/dashboard';
      navigate(next, { replace: true });
    } catch (err) {
      if (err instanceof HttpError) {
        if (err.status === 422) {
          // Invalid credentials — clear CAPTCHA state and show generic message.
          setCaptcha(null);
          setCaptchaAnswer('');
          setError({ kind: 'invalid_credentials' });
        } else if (err.status === 428 && err.body.challenge_key) {
          // CAPTCHA required — store challenge for next submission.
          setCaptcha({ key: err.body.challenge_key, question: err.body.challenge ?? '' });
          setCaptchaAnswer('');
          setError({ kind: 'captcha_required', captcha: { key: err.body.challenge_key, question: err.body.challenge ?? '' } });
        } else if (err.status === 423) {
          setCaptcha(null);
          setError({ kind: 'account_locked', seconds: err.body.retry_after_seconds ?? 900 });
        } else {
          setError({ kind: 'generic', message: err.body.detail ?? err.body.error });
        }
      } else {
        setError({ kind: 'network_error' });
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div style={S.page}>
      <div style={S.card}>
        <div style={S.title}>Library Management System</div>
        <div style={S.subtitle}>Staff sign-in — local credentials only</div>

        {/* Error and status messages */}
        {error?.kind === 'invalid_credentials' && (
          <div style={S.errorBox} role="alert">
            Invalid username or password.
          </div>
        )}
        {error?.kind === 'account_locked' && (
          <div style={S.errorBox} role="alert">
            Account locked. Try again in {formatSeconds(error.seconds)}.
          </div>
        )}
        {error?.kind === 'network_error' && (
          <div style={S.errorBox} role="alert">
            Could not reach the server. Verify the backend is running.
          </div>
        )}
        {error?.kind === 'generic' && (
          <div style={S.errorBox} role="alert">
            {error.message}
          </div>
        )}

        {/* Session expired message (redirected here after 401) */}
        {searchParams.get('reason') === 'expired' && !error && (
          <div style={S.warnBox} role="status">
            Your session has expired. Please sign in again.
          </div>
        )}

        <form onSubmit={handleSubmit} noValidate>
          <label style={S.label} htmlFor="username">Username</label>
          <input
            id="username"
            type="text"
            autoComplete="username"
            autoFocus
            required
            style={S.input}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            disabled={submitting}
          />

          <label style={S.label} htmlFor="password">Password</label>
          <input
            id="password"
            type="password"
            autoComplete="current-password"
            required
            style={S.input}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={submitting}
          />

          {/* CAPTCHA section — shown after 3 failed login attempts */}
          {captcha && (
            <>
              <div style={S.captchaBox} role="status">
                <strong>Security check required</strong>
                <br />
                {captcha.question}
              </div>
              {error?.kind === 'captcha_required' && (
                <div style={S.errorBox} role="alert">
                  Incorrect answer. Please try again.
                </div>
              )}
              <label style={S.label} htmlFor="captcha_answer">Answer</label>
              <input
                id="captcha_answer"
                type="text"
                inputMode="numeric"
                required
                style={S.input}
                value={captchaAnswer}
                onChange={(e) => setCaptchaAnswer(e.target.value)}
                disabled={submitting}
                placeholder="Enter the number"
              />
            </>
          )}

          <button
            type="submit"
            style={{
              ...S.submitBtn,
              opacity: submitting ? 0.7 : 1,
              cursor: submitting ? 'not-allowed' : 'pointer',
            }}
            disabled={submitting}
          >
            {submitting ? 'Signing in…' : 'Sign in'}
          </button>
        </form>
      </div>
    </div>
  );
}
