// HoldingsListPage — searchable, filterable, paginated list of library holdings.

import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { holdingsApi, Holding, PageResult } from '../../api/holdings';

// ── Status badge ──────────────────────────────────────────────────────────────

function ActiveBadge({ active }: { active: boolean }) {
  return (
    <span style={{
      display: 'inline-block',
      padding: '0.125rem 0.5rem',
      borderRadius: '9999px',
      fontSize: '0.6875rem',
      fontWeight: 600,
      background: active ? '#d1fae5' : '#f3f4f6',
      color: active ? '#065f46' : '#6b7280',
    }}>
      {active ? 'Active' : 'Inactive'}
    </span>
  );
}

const EMPTY_FILTERS = { search: '', category: '', activeOnly: false };

export default function HoldingsListPage() {
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('holdings:write');

  const [result, setResult] = useState<PageResult<Holding> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState(EMPTY_FILTERS);
  const [pendingFilters, setPendingFilters] = useState(EMPTY_FILTERS);
  const [page, setPage] = useState(1);
  const [perPage] = useState(20);

  const abortRef = useRef<AbortController | null>(null);

  const fetchHoldings = useCallback(
    async (activeFilters: typeof EMPTY_FILTERS, activePage: number, activePerPage: number) => {
      abortRef.current?.abort();
      const ac = new AbortController();
      abortRef.current = ac;

      setLoading(true);
      setError(null);
      try {
        const data = await holdingsApi.list({
          search: activeFilters.search || undefined,
          category: activeFilters.category || undefined,
          active: activeFilters.activeOnly ? true : undefined,
          page: activePage,
          per_page: activePerPage,
        });
        if (!ac.signal.aborted) setResult(data);
      } catch (err: unknown) {
        if (!ac.signal.aborted) {
          setError(err instanceof Error ? err.message : 'Failed to load holdings');
        }
      } finally {
        if (!ac.signal.aborted) setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    fetchHoldings(filters, page, perPage);
  }, [filters, page, perPage, fetchHoldings]);

  function handleApply() {
    setFilters(pendingFilters);
    setPage(1);
  }

  function handleClear() {
    setPendingFilters(EMPTY_FILTERS);
    setFilters(EMPTY_FILTERS);
    setPage(1);
  }

  return (
    <div>
      {/* Page header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
        <div>
          <div style={{ fontSize: '1.125rem', fontWeight: 700, color: '#1a1a2e' }}>Holdings</div>
          {result && (
            <div style={{ fontSize: '0.75rem', color: '#6b7280', marginTop: '0.125rem' }}>
              {result.total.toLocaleString()} record{result.total !== 1 ? 's' : ''}
            </div>
          )}
        </div>
        {canWrite && (
          <button
            onClick={() => navigate('/holdings/new')}
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
            + Add Holding
          </button>
        )}
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: '0.75rem', flexWrap: 'wrap', marginBottom: '1rem', alignItems: 'flex-end' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
          <label style={{ fontSize: '0.75rem', fontWeight: 600, color: '#374151' }}>Search</label>
          <input
            type="text"
            placeholder="Title, author, or ISBN"
            value={pendingFilters.search}
            onChange={(e) => setPendingFilters((f) => ({ ...f, search: e.target.value }))}
            onKeyDown={(e) => e.key === 'Enter' && handleApply()}
            style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem', width: '220px' }}
          />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
          <label style={{ fontSize: '0.75rem', fontWeight: 600, color: '#374151' }}>Category</label>
          <input
            type="text"
            placeholder="e.g. Fiction"
            value={pendingFilters.category}
            onChange={(e) => setPendingFilters((f) => ({ ...f, category: e.target.value }))}
            onKeyDown={(e) => e.key === 'Enter' && handleApply()}
            style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem', width: '160px' }}
          />
        </div>
        <label style={{ display: 'flex', alignItems: 'center', gap: '0.375rem', fontSize: '0.875rem', cursor: 'pointer', paddingBottom: '0.125rem' }}>
          <input
            type="checkbox"
            checked={pendingFilters.activeOnly}
            onChange={(e) => setPendingFilters((f) => ({ ...f, activeOnly: e.target.checked }))}
          />
          Active only
        </label>
        <button
          onClick={handleApply}
          disabled={loading}
          style={{ padding: '0.375rem 0.875rem', background: '#2563eb', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '0.875rem', fontWeight: 600, opacity: loading ? 0.6 : 1 }}
        >
          Apply
        </button>
        <button
          onClick={handleClear}
          style={{ padding: '0.375rem 0.875rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: '4px', cursor: 'pointer', fontSize: '0.875rem' }}
        >
          Clear
        </button>
      </div>

      {/* Error */}
      {error && (
        <div style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.625rem 0.875rem', fontSize: '0.8125rem', color: '#dc2626', marginBottom: '0.75rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span>{error}</span>
          <button onClick={() => fetchHoldings(filters, page, perPage)} style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', fontSize: '0.8125rem' }}>Retry</button>
        </div>
      )}

      {/* Table */}
      <div style={{ border: '1px solid #e5e7eb', borderRadius: '6px', overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
          <thead>
            <tr style={{ background: '#f9fafb' }}>
              {['Title', 'Author', 'ISBN', 'Category', 'Status'].map((h) => (
                <th key={h} style={{ padding: '0.5rem 0.875rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', color: '#6b7280', borderBottom: '2px solid #e5e7eb' }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {loading && (
              <tr>
                <td colSpan={5} style={{ padding: '2rem', textAlign: 'center', color: '#6b7280' }}>Loading…</td>
              </tr>
            )}
            {!loading && (!result || result.items.length === 0) && (
              <tr>
                <td colSpan={5} style={{ padding: '2rem', textAlign: 'center', color: '#6b7280' }}>
                  <div style={{ fontWeight: 600, marginBottom: '0.25rem' }}>No holdings found</div>
                  <div style={{ fontSize: '0.75rem' }}>
                    {filters.search || filters.category ? 'Try adjusting your filters.' : 'Add the first holding to get started.'}
                  </div>
                </td>
              </tr>
            )}
            {!loading && result && result.items.map((h) => (
              <tr
                key={h.id}
                onClick={() => navigate(`/holdings/${h.id}`)}
                style={{ borderBottom: '1px solid #f3f4f6', cursor: 'pointer', transition: 'background 0.1s' }}
                onMouseEnter={(e) => (e.currentTarget.style.background = '#f9fafb')}
                onMouseLeave={(e) => (e.currentTarget.style.background = '')}
              >
                <td style={{ padding: '0.5rem 0.875rem', fontWeight: 500, color: '#1a1a2e' }}>{h.title}</td>
                <td style={{ padding: '0.5rem 0.875rem', color: '#374151' }}>{h.author ?? '—'}</td>
                <td style={{ padding: '0.5rem 0.875rem', fontFamily: 'monospace', fontSize: '0.75rem', color: '#6b7280' }}>{h.isbn ?? '—'}</td>
                <td style={{ padding: '0.5rem 0.875rem', color: '#374151' }}>{h.category ?? '—'}</td>
                <td style={{ padding: '0.5rem 0.875rem' }}><ActiveBadge active={h.is_active} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {result && result.total_pages > 1 && (
        <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'space-between', alignItems: 'center', paddingTop: '0.75rem', fontSize: '0.8125rem', color: '#6b7280' }}>
          <span>Page {page} of {result.total_pages} ({result.total.toLocaleString()} total)</span>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button disabled={page <= 1} onClick={() => setPage(page - 1)} style={{ padding: '0.25rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', cursor: page <= 1 ? 'default' : 'pointer', opacity: page <= 1 ? 0.4 : 1, background: '#fff' }}>‹ Prev</button>
            <button disabled={page >= result.total_pages} onClick={() => setPage(page + 1)} style={{ padding: '0.25rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', cursor: page >= result.total_pages ? 'default' : 'pointer', opacity: page >= result.total_pages ? 0.4 : 1, background: '#fff' }}>Next ›</button>
          </div>
        </div>
      )}
    </div>
  );
}
