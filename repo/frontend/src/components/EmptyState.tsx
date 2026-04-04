// EmptyState is shown when a query returns zero results.
// Accepts an optional action button for common follow-on tasks (e.g. "Add reader").

interface EmptyStateProps {
  message?: string;
  detail?: string;
  actionLabel?: string;
  onAction?: () => void;
}

export default function EmptyState({
  message = 'No results',
  detail,
  actionLabel,
  onAction,
}: EmptyStateProps) {
  return (
    <div
      data-testid="empty-state"
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        padding: '3rem 1rem',
        gap: '0.5rem',
        color: '#9ca3af',
        fontSize: '0.875rem',
        textAlign: 'center',
      }}
    >
      <div style={{ fontSize: '2rem', marginBottom: '0.25rem' }}>—</div>
      <div style={{ color: '#374151', fontWeight: 500 }}>{message}</div>
      {detail && <div style={{ fontSize: '0.8125rem' }}>{detail}</div>}
      {actionLabel && onAction && (
        <button
          onClick={onAction}
          style={{
            marginTop: '0.5rem',
            padding: '0.375rem 1rem',
            background: '#2563eb',
            color: '#fff',
            border: 'none',
            borderRadius: '4px',
            cursor: 'pointer',
            fontSize: '0.8125rem',
          }}
        >
          {actionLabel}
        </button>
      )}
    </div>
  );
}
