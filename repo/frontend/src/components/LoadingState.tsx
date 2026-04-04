// LoadingState shows a spinner for in-progress data fetches.
// Use inside DataTable or as a full-page overlay.

interface LoadingStateProps {
  message?: string;
  /** If true, centers vertically within a full container. */
  fullHeight?: boolean;
}

export default function LoadingState({
  message = 'Loading…',
  fullHeight = false,
}: LoadingStateProps) {
  return (
    <div
      data-testid="loading-state"
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        padding: fullHeight ? '0' : '3rem 1rem',
        height: fullHeight ? '100%' : undefined,
        gap: '0.75rem',
        color: '#6b7280',
        fontSize: '0.875rem',
      }}
    >
      <div
        aria-hidden="true"
        style={{
          width: '1.75rem',
          height: '1.75rem',
          border: '3px solid #e5e7eb',
          borderTopColor: '#2563eb',
          borderRadius: '50%',
          animation: 'lms-spin 0.7s linear infinite',
        }}
      />
      <style>{`@keyframes lms-spin { to { transform: rotate(360deg); } }`}</style>
      <span>{message}</span>
    </div>
  );
}
