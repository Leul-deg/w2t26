// StepUpModal requires the user to re-enter their password before a sensitive
// action is performed (e.g. revealing encrypted reader fields).
//
// The step-up check is always server-validated via POST /api/v1/auth/stepup.
// The client never decides whether the password is correct — the server does.
//
// Usage:
//   <StepUpModal
//     title="Reveal sensitive field"
//     description="Enter your password to view this field."
//     onSuccess={() => setRevealed(true)}
//     onCancel={() => setShowModal(false)}
//   />

import { FormEvent, useState } from 'react';
import { apiClient, HttpError } from '../api/client';

interface StepUpModalProps {
  title?: string;
  description?: string;
  onSuccess: () => void;
  onCancel: () => void;
}

export default function StepUpModal({
  title = 'Confirm your identity',
  description = 'Enter your password to continue.',
  onSuccess,
  onCancel,
}: StepUpModalProps) {
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);

    try {
      await apiClient.post<{ ok: boolean }>('/auth/stepup', { password });
      // Server validated the password — call the success handler.
      onSuccess();
    } catch (err) {
      if (err instanceof HttpError && err.status === 401) {
        setError('Incorrect password. Please try again.');
      } else if (err instanceof HttpError && err.status === 422) {
        setError(err.body.detail ?? 'Password is required.');
      } else {
        setError('Could not verify your identity. Please try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    // Backdrop
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="stepup-title"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.4)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 10000,
        fontFamily: 'system-ui, -apple-system, sans-serif',
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onCancel();
      }}
    >
      {/* Dialog box */}
      <div
        style={{
          background: '#fff',
          borderRadius: '6px',
          padding: '1.5rem',
          width: '100%',
          maxWidth: '380px',
          boxShadow: '0 8px 32px rgba(0,0,0,0.18)',
        }}
      >
        <div
          id="stepup-title"
          style={{ fontWeight: 700, fontSize: '1rem', marginBottom: '0.375rem', color: '#1a1a2e' }}
        >
          {title}
        </div>
        <div style={{ fontSize: '0.8125rem', color: '#6b7280', marginBottom: '1.25rem' }}>
          {description}
        </div>

        {error && (
          <div
            role="alert"
            style={{
              background: '#fef2f2',
              border: '1px solid #fca5a5',
              borderRadius: '4px',
              padding: '0.625rem 0.875rem',
              fontSize: '0.8125rem',
              color: '#dc2626',
              marginBottom: '1rem',
            }}
          >
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <label
            htmlFor="stepup-password"
            style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 500, marginBottom: '0.375rem', color: '#374151' }}
          >
            Password
          </label>
          <input
            id="stepup-password"
            type="password"
            autoComplete="current-password"
            autoFocus
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={submitting}
            style={{
              display: 'block',
              width: '100%',
              padding: '0.5rem 0.75rem',
              border: '1px solid #d1d5db',
              borderRadius: '4px',
              fontSize: '0.875rem',
              marginBottom: '1.25rem',
              boxSizing: 'border-box',
              color: '#1a1a2e',
            }}
          />

          <div style={{ display: 'flex', gap: '0.625rem', justifyContent: 'flex-end' }}>
            <button
              type="button"
              onClick={onCancel}
              style={{
                padding: '0.5rem 1rem',
                background: '#fff',
                color: '#374151',
                border: '1px solid #d1d5db',
                borderRadius: '4px',
                cursor: 'pointer',
                fontSize: '0.875rem',
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting || password.length === 0}
              style={{
                padding: '0.5rem 1rem',
                background: '#2563eb',
                color: '#fff',
                border: 'none',
                borderRadius: '4px',
                cursor: submitting ? 'not-allowed' : 'pointer',
                fontSize: '0.875rem',
                fontWeight: 600,
                opacity: submitting ? 0.7 : 1,
              }}
            >
              {submitting ? 'Verifying…' : 'Confirm'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
