// SessionTimeoutBanner is shown when the user has been idle for 25 minutes.
// It offers a "Stay signed in" button that resets the inactivity timer and
// extends the server session (via GET /auth/me).

import { useState } from 'react';
import { apiClient } from '../api/client';

interface SessionTimeoutBannerProps {
  /** Called when the user chooses to extend the session. */
  onExtend: () => void;
  /** Called when the user explicitly dismisses/signs out early. */
  onSignOut: () => void;
}

export default function SessionTimeoutBanner({ onExtend, onSignOut }: SessionTimeoutBannerProps) {
  const [extending, setExtending] = useState(false);

  async function handleExtend() {
    setExtending(true);
    try {
      // GET /auth/me is a low-cost authenticated call that extends the server session.
      await apiClient.get('/auth/me');
    } catch {
      // If the server returns 401, the global 401 handler fires and we'll be
      // redirected to login. Nothing more to do here.
    } finally {
      setExtending(false);
    }
    onExtend();
  }

  return (
    <div
      role="alert"
      aria-live="assertive"
      data-testid="session-timeout-banner"
      style={{
        position: 'fixed',
        bottom: '1.5rem',
        right: '1.5rem',
        background: '#1e2235',
        color: '#e2e8f0',
        border: '1px solid #f59e0b',
        borderRadius: '6px',
        padding: '1rem 1.25rem',
        maxWidth: '360px',
        boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
        zIndex: 9999,
        fontFamily: 'system-ui, -apple-system, sans-serif',
        fontSize: '0.875rem',
      }}
    >
      <div style={{ fontWeight: 600, marginBottom: '0.375rem', color: '#fbbf24' }}>
        Session expiring soon
      </div>
      <div style={{ color: '#94a3b8', marginBottom: '1rem', fontSize: '0.8125rem' }}>
        Your session will expire due to inactivity. Stay signed in to continue working.
      </div>
      <div style={{ display: 'flex', gap: '0.625rem' }}>
        <button
          onClick={handleExtend}
          disabled={extending}
          style={{
            padding: '0.375rem 0.875rem',
            background: '#2563eb',
            color: '#fff',
            border: 'none',
            borderRadius: '4px',
            cursor: extending ? 'not-allowed' : 'pointer',
            fontSize: '0.8125rem',
            fontWeight: 600,
            opacity: extending ? 0.7 : 1,
          }}
        >
          {extending ? 'Extending…' : 'Stay signed in'}
        </button>
        <button
          onClick={onSignOut}
          style={{
            padding: '0.375rem 0.875rem',
            background: 'transparent',
            color: '#94a3b8',
            border: '1px solid #374151',
            borderRadius: '4px',
            cursor: 'pointer',
            fontSize: '0.8125rem',
          }}
        >
          Sign out
        </button>
      </div>
    </div>
  );
}
