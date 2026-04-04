// ErrorState shows an error message with an optional retry button.
// Use inside DataTable or as a standalone section error.

interface ErrorStateProps {
  message?: string;
  onRetry?: () => void;
}

export default function ErrorState({
  message = 'Something went wrong. Please try again.',
  onRetry,
}: ErrorStateProps) {
  return (
    <div
      data-testid="error-state"
      role="alert"
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        padding: '3rem 1rem',
        gap: '0.75rem',
        color: '#6b7280',
        fontSize: '0.875rem',
        textAlign: 'center',
      }}
    >
      <div style={{ fontSize: '1.5rem', color: '#dc2626' }}>!</div>
      <div style={{ color: '#374151', fontWeight: 500 }}>Error</div>
      <div>{message}</div>
      {onRetry && (
        <button
          onClick={onRetry}
          style={{
            marginTop: '0.25rem',
            padding: '0.375rem 1rem',
            background: '#2563eb',
            color: '#fff',
            border: 'none',
            borderRadius: '4px',
            cursor: 'pointer',
            fontSize: '0.8125rem',
          }}
        >
          Retry
        </button>
      )}
    </div>
  );
}
