// ReadersListPage — searchable, filterable, paginated list of library readers.
//
// Security notes:
//  - Sensitive fields are always masked in the list view (server returns "••••••").
//  - Status change and create actions require readers:write permission.
//  - The page gracefully degrades when the user only has readers:read.

import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { readersApi, Reader, ReaderStatus, PageResult } from '../../api/readers';
import DataTable, { Column } from '../../components/DataTable';
import FilterBar from '../../components/FilterBar';
import Pagination from '../../components/Pagination';

// Status badge colours
const STATUS_COLOUR: Record<string, { bg: string; color: string }> = {
  active:               { bg: '#d1fae5', color: '#065f46' },
  frozen:               { bg: '#dbeafe', color: '#1e40af' },
  blacklisted:          { bg: '#fee2e2', color: '#991b1b' },
  pending_verification: { bg: '#fef3c7', color: '#92400e' },
};

function StatusBadge({ code }: { code: string }) {
  const style = STATUS_COLOUR[code] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{
      display: 'inline-block',
      padding: '0.125rem 0.5rem',
      borderRadius: '9999px',
      fontSize: '0.6875rem',
      fontWeight: 600,
      background: style.bg,
      color: style.color,
      textTransform: 'capitalize',
    }}>
      {code.replace('_', ' ')}
    </span>
  );
}

const EMPTY_FILTERS = { search: '', status: '' };

export default function ReadersListPage() {
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('readers:write');

  const [result, setResult] = useState<PageResult<Reader> | null>(null);
  const [statuses, setStatuses] = useState<ReaderStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState(EMPTY_FILTERS);
  const [page, setPage] = useState(1);
  const [perPage, setPerPage] = useState(20);

  // Track the pending filter values (not yet "applied")
  const [pendingFilters, setPendingFilters] = useState(EMPTY_FILTERS);

  const abortRef = useRef<AbortController | null>(null);

  const fetchReaders = useCallback(
    async (activeFilters: typeof EMPTY_FILTERS, activePage: number, activePerPage: number) => {
      abortRef.current?.abort();
      const ac = new AbortController();
      abortRef.current = ac;

      setLoading(true);
      setError(null);
      try {
        const data = await readersApi.list({
          search: activeFilters.search || undefined,
          status: activeFilters.status || undefined,
          page: activePage,
          per_page: activePerPage,
        });
        if (!ac.signal.aborted) setResult(data);
      } catch (err: unknown) {
        if (!ac.signal.aborted) {
          setError(err instanceof Error ? err.message : 'Failed to load readers');
        }
      } finally {
        if (!ac.signal.aborted) setLoading(false);
      }
    },
    [],
  );

  // Load statuses once on mount
  useEffect(() => {
    readersApi.listStatuses().then(setStatuses).catch(() => {});
  }, []);

  // Fetch when active filters / page / perPage change
  useEffect(() => {
    fetchReaders(filters, page, perPage);
  }, [filters, page, perPage, fetchReaders]);

  function handleApply() {
    setFilters(pendingFilters);
    setPage(1);
  }

  function handleClear() {
    setPendingFilters(EMPTY_FILTERS);
    setFilters(EMPTY_FILTERS);
    setPage(1);
  }

  const columns: Column<Reader>[] = [
    {
      key: 'reader_number',
      header: 'Reader #',
      width: '130px',
      render: (r) => (
        <span style={{ fontFamily: 'monospace', fontSize: '0.8125rem' }}>{r.reader_number}</span>
      ),
    },
    {
      key: 'name',
      header: 'Name',
      render: (r) => {
        const preferred = r.preferred_name ? ` (${r.preferred_name})` : '';
        return `${r.last_name}, ${r.first_name}${preferred}`;
      },
    },
    {
      key: 'status',
      header: 'Status',
      width: '140px',
      render: (r) => <StatusBadge code={r.status_code} />,
    },
    {
      key: 'registered',
      header: 'Registered',
      width: '110px',
      render: (r) => new Date(r.registered_at).toLocaleDateString(),
    },
  ];

  const statusOptions = statuses.map((s) => ({ value: s.code, label: s.description }));

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
        <div>
          <div style={{ fontSize: '1.125rem', fontWeight: 700, color: '#1a1a2e' }}>Readers</div>
          {result && (
            <div style={{ fontSize: '0.75rem', color: '#6b7280', marginTop: '0.125rem' }}>
              {result.total.toLocaleString()} record{result.total !== 1 ? 's' : ''}
            </div>
          )}
        </div>
        {canWrite && (
          <button
            onClick={() => navigate('/readers/new')}
            style={{
              padding: '0.5rem 1rem',
              background: '#2563eb',
              color: '#fff',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontSize: '0.875rem',
              fontWeight: 600,
            }}
          >
            + New Reader
          </button>
        )}
      </div>

      {/* Filters */}
      <FilterBar
        fields={[
          { name: 'search', label: 'Search', type: 'text', placeholder: 'Name or reader #' },
          { name: 'status', label: 'Status', type: 'select', options: statusOptions },
        ]}
        values={pendingFilters}
        onChange={(name, value) => setPendingFilters((f) => ({ ...f, [name]: value }))}
        onApply={handleApply}
        onClear={handleClear}
        loading={loading}
      />

      {/* Table */}
      <DataTable
        columns={columns}
        rows={result?.items ?? []}
        rowKey={(r) => r.id}
        loading={loading}
        error={error ?? undefined}
        onRetry={() => fetchReaders(filters, page, perPage)}
        onRowClick={(r) => navigate(`/readers/${r.id}`)}
        emptyMessage="No readers found"
        emptyDetail={filters.search || filters.status ? 'Try adjusting your filters.' : 'Create the first reader to get started.'}
        emptyActionLabel={canWrite ? 'New Reader' : undefined}
        onEmptyAction={canWrite ? () => navigate('/readers/new') : undefined}
        caption="Readers list"
      />

      {/* Pagination */}
      {result && result.total > 0 && (
        <Pagination
          page={page}
          totalPages={result.total_pages}
          pageSize={perPage}
          totalItems={result.total}
          onPageChange={setPage}
          onPageSizeChange={(size) => { setPerPage(size); setPage(1); }}
        />
      )}
    </div>
  );
}
