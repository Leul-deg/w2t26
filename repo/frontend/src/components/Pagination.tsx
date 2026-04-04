// Pagination controls for list views.
// All state lives in the caller — this component fires callbacks only.

interface PaginationProps {
  page: number;
  totalPages: number;
  pageSize: number;
  totalItems: number;
  onPageChange: (page: number) => void;
  onPageSizeChange?: (size: number) => void;
  pageSizeOptions?: number[];
}

const BTN: React.CSSProperties = {
  padding: '0.3125rem 0.625rem',
  border: '1px solid #d1d5db',
  background: '#fff',
  color: '#374151',
  borderRadius: '4px',
  cursor: 'pointer',
  fontSize: '0.8125rem',
};

const BTN_DISABLED: React.CSSProperties = {
  ...BTN,
  opacity: 0.4,
  cursor: 'not-allowed',
};

export default function Pagination({
  page,
  totalPages,
  pageSize,
  totalItems,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = [20, 50, 100],
}: PaginationProps) {
  const start = totalItems === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, totalItems);

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0.625rem 0',
        fontSize: '0.8125rem',
        color: '#6b7280',
        flexWrap: 'wrap',
        gap: '0.5rem',
      }}
    >
      <span>
        {totalItems === 0
          ? 'No results'
          : `${start}–${end} of ${totalItems.toLocaleString()}`}
      </span>

      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        {onPageSizeChange && (
          <label style={{ display: 'flex', alignItems: 'center', gap: '0.375rem' }}>
            Rows:
            <select
              value={pageSize}
              onChange={(e) => onPageSizeChange(Number(e.target.value))}
              style={{ padding: '0.25rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.8125rem' }}
            >
              {pageSizeOptions.map((n) => (
                <option key={n} value={n}>{n}</option>
              ))}
            </select>
          </label>
        )}

        <button
          style={page <= 1 ? BTN_DISABLED : BTN}
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          aria-label="Previous page"
        >
          ‹ Prev
        </button>

        <span style={{ padding: '0 0.25rem' }}>
          {page} / {totalPages || 1}
        </span>

        <button
          style={page >= totalPages ? BTN_DISABLED : BTN}
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
          aria-label="Next page"
        >
          Next ›
        </button>
      </div>
    </div>
  );
}
