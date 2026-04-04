// UnauthorizedPage is shown when the user is authenticated but lacks permission
// for the requested resource (403 Forbidden).

import { Link } from 'react-router-dom';

export default function UnauthorizedPage() {
  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontFamily: 'system-ui, -apple-system, sans-serif',
        background: '#f3f4f6',
      }}
    >
      <div style={{ textAlign: 'center', maxWidth: '400px', padding: '2rem' }}>
        <div style={{ fontSize: '3rem', fontWeight: 700, color: '#d1d5db', marginBottom: '0.5rem' }}>
          403
        </div>
        <div style={{ fontSize: '1.125rem', fontWeight: 600, color: '#1a1a2e', marginBottom: '0.5rem' }}>
          Access denied
        </div>
        <p style={{ fontSize: '0.875rem', color: '#6b7280', marginBottom: '1.5rem' }}>
          You do not have permission to view this page. Contact your administrator if you
          believe this is an error.
        </p>
        <Link
          to="/dashboard"
          style={{
            display: 'inline-block',
            padding: '0.5rem 1.25rem',
            background: '#2563eb',
            color: '#fff',
            textDecoration: 'none',
            borderRadius: '4px',
            fontSize: '0.875rem',
            fontWeight: 500,
          }}
        >
          Back to dashboard
        </Link>
      </div>
    </div>
  );
}
