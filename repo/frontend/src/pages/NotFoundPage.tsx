// NotFoundPage is shown for any route that does not match the route table.

import { Link } from 'react-router-dom';

export default function NotFoundPage() {
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
          404
        </div>
        <div style={{ fontSize: '1.125rem', fontWeight: 600, color: '#1a1a2e', marginBottom: '0.5rem' }}>
          Page not found
        </div>
        <p style={{ fontSize: '0.875rem', color: '#6b7280', marginBottom: '1.5rem' }}>
          The page you requested does not exist.
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
