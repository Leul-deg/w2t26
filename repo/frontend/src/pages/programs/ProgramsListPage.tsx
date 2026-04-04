// ProgramsListPage — searchable, filterable list of library programs.
//
// Shows: title, status, dates, capacity. Enroll action available inline for
// staff with enrollments:write. Create requires programs:write.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { programsApi, Program } from '../../api/programs';
import type { PageResult } from '../../api/readers';

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  draft:     { bg: '#f3f4f6', color: '#374151' },
  published: { bg: '#d1fae5', color: '#065f46' },
  cancelled: { bg: '#fee2e2', color: '#991b1b' },
  completed: { bg: '#dbeafe', color: '#1e40af' },
};

function StatusBadge({ status }: { status: string }) {
  const s = STATUS_STYLE[status] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{
      display: 'inline-block', padding: '0.125rem 0.5rem', borderRadius: '9999px',
      fontSize: '0.6875rem', fontWeight: 600, background: s.bg, color: s.color,
    }}>
      {status}
    </span>
  );
}

function fmtDate(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}

export default function ProgramsListPage() {
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('programs:write');

  const [result, setResult] = useState<PageResult<Program> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [page, setPage] = useState(1);

  const fetchPrograms = useCallback(async (s: string, st: string, p: number) => {
    setLoading(true);
    setError(null);
    try {
      const data = await programsApi.list({
        search: s || undefined,
        status: st || undefined,
        page: p,
        per_page: 20,
      });
      setResult(data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load programs');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchPrograms(search, status, page); }, []);

  const handleSearch = () => {
    setPage(1);
    fetchPrograms(search, status, 1);
  };

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: '1.5rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Programs</h1>
        {canWrite && (
          <button
            onClick={() => navigate('/programs/new')}
            style={{ padding: '0.5rem 1rem', background: '#4f46e5', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}
          >
            + New program
          </button>
        )}
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: '0.75rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          placeholder="Search title…"
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: 220 }}
        />
        <select
          value={status}
          onChange={(e) => { setStatus(e.target.value); setPage(1); fetchPrograms(search, e.target.value, 1); }}
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem' }}
        >
          <option value="">All statuses</option>
          <option value="draft">Draft</option>
          <option value="published">Published</option>
          <option value="cancelled">Cancelled</option>
          <option value="completed">Completed</option>
        </select>
        <button onClick={handleSearch} style={{ padding: '0.375rem 0.75rem', background: '#4f46e5', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}>
          Search
        </button>
      </div>

      {loading && <p style={{ color: '#6b7280' }}>Loading…</p>}
      {error && <p style={{ color: '#dc2626' }}>{error}</p>}
      {result && result.items.length === 0 && !loading && (
        <p style={{ color: '#6b7280' }}>No programs found.</p>
      )}

      {result && result.items.length > 0 && (
        <>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr style={{ background: '#f3f4f6', borderBottom: '1px solid #e5e7eb' }}>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Title</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Status</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Starts</th>
                  <th style={{ textAlign: 'right', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Capacity</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Category</th>
                  <th style={{ padding: '0.5rem 0.75rem' }}></th>
                </tr>
              </thead>
              <tbody>
                {result.items.map((prog) => (
                  <tr
                    key={prog.id}
                    style={{ borderBottom: '1px solid #f3f4f6', cursor: 'pointer' }}
                    onClick={() => navigate(`/programs/${prog.id}`)}
                  >
                    <td style={{ padding: '0.5rem 0.75rem', fontWeight: 500 }}>{prog.title}</td>
                    <td style={{ padding: '0.5rem 0.75rem' }}><StatusBadge status={prog.status} /></td>
                    <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280', fontSize: '0.75rem' }}>{fmtDate(prog.starts_at)}</td>
                    <td style={{ padding: '0.5rem 0.75rem', textAlign: 'right', fontFamily: 'monospace' }}>{prog.capacity}</td>
                    <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280' }}>{prog.category ?? '—'}</td>
                    <td style={{ padding: '0.5rem 0.75rem' }}>
                      <button
                        onClick={(e) => { e.stopPropagation(); navigate(`/programs/${prog.id}`); }}
                        style={{ fontSize: '0.75rem', color: '#4f46e5', background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline' }}
                      >
                        View
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {result.total_pages > 1 && (
            <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem', alignItems: 'center' }}>
              <button onClick={() => { setPage(page - 1); fetchPrograms(search, status, page - 1); }} disabled={page <= 1} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>← Prev</button>
              <span style={{ fontSize: '0.8125rem' }}>Page {page} of {result.total_pages}</span>
              <button onClick={() => { setPage(page + 1); fetchPrograms(search, status, page + 1); }} disabled={page >= result.total_pages} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>Next →</button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
