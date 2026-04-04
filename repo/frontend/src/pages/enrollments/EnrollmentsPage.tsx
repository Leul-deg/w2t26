import { useState } from 'react';
import { programsApi, Enrollment, EnrollmentStatus } from '../../api/programs';
import { useAuth } from '../../auth/AuthContext';

const STATUS_COLORS: Record<EnrollmentStatus, { bg: string; text: string }> = {
  pending:    { bg: '#fef3c7', text: '#92400e' },
  confirmed:  { bg: '#dcfce7', text: '#166534' },
  waitlisted: { bg: '#e0f2fe', text: '#075985' },
  cancelled:  { bg: '#fee2e2', text: '#991b1b' },
  completed:  { bg: '#f3f4f6', text: '#374151' },
  no_show:    { bg: '#ede9fe', text: '#5b21b6' },
};

export default function EnrollmentsPage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('enrollments:write');

  // Filter by reader or program ID
  const [filterMode, setFilterMode] = useState<'reader' | 'program'>('reader');
  const [filterID, setFilterID] = useState('');
  const [submitted, setSubmitted] = useState(false);

  const [items, setItems] = useState<Enrollment[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [dropping, setDropping] = useState<string | null>(null);
  const [dropReason, setDropReason] = useState('');

  async function load(id: string, mode: 'reader' | 'program', p: number) {
    if (!id.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const params = { page: p, per_page: 20 };
      const res = mode === 'reader'
        ? await programsApi.listEnrollmentsByReader(id.trim(), params)
        : await programsApi.listEnrollmentsByProgram(id.trim(), params);
      setItems(res.items ?? []);
      setTotal(res.total);
      setTotalPages(res.total_pages);
      setPage(p);
    } catch (e: any) {
      setError(e?.message ?? 'Failed to load enrollments');
      setItems([]);
    } finally {
      setLoading(false);
    }
  }

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    setSubmitted(true);
    load(filterID, filterMode, 1);
  }

  async function handleDrop(enrollmentID: string, readerID: string) {
    setDropping(enrollmentID);
    try {
      await programsApi.dropEnrollment(enrollmentID, readerID, dropReason || undefined);
      setDropReason('');
      setDropping(null);
      load(filterID, filterMode, page);
    } catch (e: any) {
      setError(e?.message ?? 'Failed to drop enrollment');
      setDropping(null);
    }
  }

  const colStyle: React.CSSProperties = {
    padding: '0.5rem 0.75rem',
    textAlign: 'left',
    borderBottom: '1px solid #e5e7eb',
    fontSize: '0.8125rem',
    whiteSpace: 'nowrap',
  };
  const thStyle: React.CSSProperties = {
    ...colStyle,
    background: '#f9fafb',
    fontWeight: 600,
    color: '#374151',
  };

  return (
    <div>
      <div style={{ fontSize: '1.125rem', fontWeight: 700, color: '#1a1a2e', marginBottom: '1rem' }}>
        Enrollments
      </div>

      {/* Search form */}
      <form onSubmit={handleSearch} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
        <select
          value={filterMode}
          onChange={(e) => { setFilterMode(e.target.value as 'reader' | 'program'); setSubmitted(false); setItems([]); }}
          style={{
            padding: '0.375rem 0.5rem',
            border: '1px solid #d1d5db',
            borderRadius: '4px',
            fontSize: '0.8125rem',
          }}
        >
          <option value="reader">By Reader ID</option>
          <option value="program">By Program ID</option>
        </select>
        <input
          type="text"
          placeholder={filterMode === 'reader' ? 'Reader UUID…' : 'Program UUID…'}
          value={filterID}
          onChange={(e) => setFilterID(e.target.value)}
          style={{
            padding: '0.375rem 0.5rem',
            border: '1px solid #d1d5db',
            borderRadius: '4px',
            fontSize: '0.8125rem',
            width: '280px',
          }}
        />
        <button
          type="submit"
          style={{
            background: '#1a1a2e',
            color: '#fff',
            border: 'none',
            borderRadius: '4px',
            padding: '0.375rem 0.875rem',
            fontSize: '0.8125rem',
            cursor: 'pointer',
          }}
        >
          Search
        </button>
      </form>

      {loading && <div style={{ color: '#6b7280', fontSize: '0.875rem' }}>Loading…</div>}
      {error && <div style={{ color: '#b91c1c', fontSize: '0.875rem', marginBottom: '0.5rem' }}>{error}</div>}

      {!loading && submitted && !error && (
        <>
          <div style={{ fontSize: '0.75rem', color: '#6b7280', marginBottom: '0.5rem' }}>
            {total} enrollment{total !== 1 ? 's' : ''}
          </div>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr>
                  <th style={thStyle}>{filterMode === 'reader' ? 'Program ID' : 'Reader ID'}</th>
                  <th style={thStyle}>Status</th>
                  <th style={thStyle}>Channel</th>
                  <th style={thStyle}>Enrolled At</th>
                  {canWrite && <th style={thStyle}>Actions</th>}
                </tr>
              </thead>
              <tbody>
                {items.map((e) => {
                  const colors = STATUS_COLORS[e.status] ?? { bg: '#f3f4f6', text: '#374151' };
                  const isDroppingThis = dropping === e.id;
                  return (
                    <tr key={e.id} style={{ background: '#fff' }}>
                      <td style={{ ...colStyle, fontFamily: 'monospace', fontSize: '0.75rem' }}>
                        {filterMode === 'reader' ? e.program_id : e.reader_id}
                      </td>
                      <td style={colStyle}>
                        <span style={{
                          padding: '0.125rem 0.5rem',
                          borderRadius: '9999px',
                          fontSize: '0.75rem',
                          background: colors.bg,
                          color: colors.text,
                        }}>
                          {e.status}
                        </span>
                      </td>
                      <td style={colStyle}>{e.enrollment_channel ?? '—'}</td>
                      <td style={colStyle}>{new Date(e.enrolled_at).toLocaleDateString()}</td>
                      {canWrite && (
                        <td style={colStyle}>
                          {e.status !== 'cancelled' && e.status !== 'completed' && (
                            isDroppingThis ? (
                              <div style={{ display: 'flex', gap: '0.375rem', alignItems: 'center' }}>
                                <input
                                  type="text"
                                  placeholder="Reason (optional)"
                                  value={dropReason}
                                  onChange={(ev) => setDropReason(ev.target.value)}
                                  style={{
                                    padding: '0.25rem 0.375rem',
                                    border: '1px solid #d1d5db',
                                    borderRadius: '3px',
                                    fontSize: '0.75rem',
                                    width: '160px',
                                  }}
                                />
                                <button
                                  onClick={() => handleDrop(e.id, e.reader_id)}
                                  style={{
                                    fontSize: '0.75rem',
                                    padding: '0.25rem 0.5rem',
                                    background: '#dc2626',
                                    color: '#fff',
                                    border: 'none',
                                    borderRadius: '3px',
                                    cursor: 'pointer',
                                  }}
                                >
                                  Confirm
                                </button>
                                <button
                                  onClick={() => setDropping(null)}
                                  style={{
                                    fontSize: '0.75rem',
                                    padding: '0.25rem 0.5rem',
                                    background: 'transparent',
                                    border: '1px solid #d1d5db',
                                    borderRadius: '3px',
                                    cursor: 'pointer',
                                  }}
                                >
                                  Cancel
                                </button>
                              </div>
                            ) : (
                              <button
                                onClick={() => setDropping(e.id)}
                                style={{
                                  fontSize: '0.75rem',
                                  padding: '0.25rem 0.5rem',
                                  background: 'transparent',
                                  border: '1px solid #d1d5db',
                                  borderRadius: '3px',
                                  cursor: 'pointer',
                                }}
                              >
                                Drop
                              </button>
                            )
                          )}
                        </td>
                      )}
                    </tr>
                  );
                })}
                {items.length === 0 && (
                  <tr>
                    <td
                      colSpan={canWrite ? 5 : 4}
                      style={{ ...colStyle, textAlign: 'center', color: '#6b7280', padding: '2rem' }}
                    >
                      No enrollments found.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.75rem', alignItems: 'center' }}>
              <button
                disabled={page <= 1}
                onClick={() => load(filterID, filterMode, page - 1)}
                style={{
                  padding: '0.25rem 0.625rem',
                  fontSize: '0.8125rem',
                  border: '1px solid #d1d5db',
                  borderRadius: '3px',
                  cursor: page <= 1 ? 'default' : 'pointer',
                  opacity: page <= 1 ? 0.4 : 1,
                  background: 'transparent',
                }}
              >
                ‹
              </button>
              <span style={{ fontSize: '0.8125rem', color: '#6b7280' }}>
                Page {page} of {totalPages}
              </span>
              <button
                disabled={page >= totalPages}
                onClick={() => load(filterID, filterMode, page + 1)}
                style={{
                  padding: '0.25rem 0.625rem',
                  fontSize: '0.8125rem',
                  border: '1px solid #d1d5db',
                  borderRadius: '3px',
                  cursor: page >= totalPages ? 'default' : 'pointer',
                  opacity: page >= totalPages ? 0.4 : 1,
                  background: 'transparent',
                }}
              >
                ›
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
