// DataTable is a reusable table shell for domain list views.
//
// Features:
//   - Typed column definitions with custom cell renderers
//   - Optional sort indicators (sort control is caller-managed)
//   - Loading, error, and empty states via inline slot components
//   - Row click handler for detail navigation
//   - Workstation-dense layout (compact rows, monospace identifiers)
//
// Usage:
//   <DataTable
//     columns={[
//       { key: 'reader_number', header: 'Reader #', render: r => r.reader_number },
//       { key: 'name',          header: 'Name',     render: r => `${r.first_name} ${r.last_name}` },
//     ]}
//     rows={readers}
//     rowKey={r => r.id}
//     loading={isLoading}
//     onRowClick={r => navigate(`/readers/${r.id}`)}
//   />

import LoadingState from './LoadingState';
import ErrorState from './ErrorState';
import EmptyState from './EmptyState';

export interface Column<T> {
  key: string;
  header: string;
  render: (row: T, index: number) => React.ReactNode;
  /** Hint for column width (e.g. '120px', '1fr'). Default: auto. */
  width?: string;
  /** Whether the header shows a sort indicator. Actual sort is caller-managed. */
  sortable?: boolean;
  sortDirection?: 'asc' | 'desc' | 'none';
  onSort?: () => void;
  align?: 'left' | 'right' | 'center';
}

interface DataTableProps<T> {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
  onRowClick?: (row: T) => void;
  emptyMessage?: string;
  emptyDetail?: string;
  /** Label for an optional empty-state action button. */
  emptyActionLabel?: string;
  onEmptyAction?: () => void;
  caption?: string;
}

const TH_STYLE: React.CSSProperties = {
  padding: '0.5rem 0.75rem',
  textAlign: 'left',
  fontWeight: 600,
  fontSize: '0.75rem',
  color: '#6b7280',
  borderBottom: '2px solid #e5e7eb',
  background: '#f9fafb',
  whiteSpace: 'nowrap',
  userSelect: 'none',
};

const TD_STYLE: React.CSSProperties = {
  padding: '0.5rem 0.75rem',
  fontSize: '0.8125rem',
  color: '#1a1a2e',
  borderBottom: '1px solid #f3f4f6',
  verticalAlign: 'middle',
};

export default function DataTable<T>({
  columns,
  rows,
  rowKey,
  loading = false,
  error,
  onRetry,
  onRowClick,
  emptyMessage,
  emptyDetail,
  emptyActionLabel,
  onEmptyAction,
  caption,
}: DataTableProps<T>) {
  if (loading) return <LoadingState />;
  if (error) return <ErrorState message={error} onRetry={onRetry} />;

  return (
    <div
      style={{
        background: '#fff',
        border: '1px solid #e5e7eb',
        borderRadius: '6px',
        overflow: 'hidden',
      }}
    >
      <div style={{ overflowX: 'auto' }}>
        <table
          style={{ width: '100%', borderCollapse: 'collapse', minWidth: '400px' }}
          aria-label={caption}
        >
          {caption && <caption style={{ display: 'none' }}>{caption}</caption>}
          <thead>
            <tr>
              {columns.map((col) => (
                <th
                  key={col.key}
                  style={{
                    ...TH_STYLE,
                    width: col.width,
                    textAlign: col.align ?? 'left',
                    cursor: col.sortable ? 'pointer' : undefined,
                  }}
                  onClick={col.sortable ? col.onSort : undefined}
                  aria-sort={
                    col.sortDirection === 'asc'
                      ? 'ascending'
                      : col.sortDirection === 'desc'
                        ? 'descending'
                        : undefined
                  }
                >
                  {col.header}
                  {col.sortable && col.sortDirection === 'asc' && ' ▲'}
                  {col.sortable && col.sortDirection === 'desc' && ' ▼'}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={columns.length} style={{ padding: 0 }}>
                  <EmptyState
                    message={emptyMessage ?? 'No records found'}
                    detail={emptyDetail}
                    actionLabel={emptyActionLabel}
                    onAction={onEmptyAction}
                  />
                </td>
              </tr>
            ) : (
              rows.map((row, i) => (
                <tr
                  key={rowKey(row)}
                  onClick={onRowClick ? () => onRowClick(row) : undefined}
                  style={{
                    cursor: onRowClick ? 'pointer' : undefined,
                    background: i % 2 === 0 ? '#fff' : '#fafafa',
                  }}
                  onMouseEnter={(e) => {
                    if (onRowClick) (e.currentTarget as HTMLTableRowElement).style.background = '#eff6ff';
                  }}
                  onMouseLeave={(e) => {
                    if (onRowClick) (e.currentTarget as HTMLTableRowElement).style.background = i % 2 === 0 ? '#fff' : '#fafafa';
                  }}
                >
                  {columns.map((col) => (
                    <td
                      key={col.key}
                      style={{ ...TD_STYLE, textAlign: col.align ?? 'left' }}
                    >
                      {col.render(row, i)}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
