// ModerationQueuePage — list of content items awaiting moderation.
// Requires content:moderate permission.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { moderationApi, ModerationItem } from '../../api/moderation';
import type { PageResult } from '../../api/readers';

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  pending:   { bg: '#fef3c7', color: '#92400e' },
  in_review: { bg: '#dbeafe', color: '#1e40af' },
  decided:   { bg: '#d1fae5', color: '#065f46' },
};

function StatusBadge({ status }: { status: string }) {
  const s = STATUS_STYLE[status] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{ display: 'inline-block', padding: '0.125rem 0.5rem', borderRadius: '9999px', fontSize: '0.6875rem', fontWeight: 600, background: s.bg, color: s.color }}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

export default function ModerationQueuePage() {
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canModerate = hasPermission('content:moderate');

  const [result, setResult] = useState<PageResult<ModerationItem> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState('pending');
  const [page, setPage] = useState(1);

  const fetchQueue = useCallback(async (status: string, p: number) => {
    setLoading(true);
    setError(null);
    try {
      const data = await moderationApi.listQueue({ status, page: p, per_page: 20 });
      setResult(data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load queue');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchQueue(statusFilter, 1); }, []);

  if (!canModerate) {
    return <p style={{ padding: '1.5rem', color: '#dc2626' }}>You do not have permission to view the moderation queue.</p>;
  }

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: '1.5rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Moderation Queue</h1>
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: '0.75rem', marginBottom: '1rem' }}>
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setPage(1); fetchQueue(e.target.value, 1); }}
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem' }}
        >
          <option value="pending">Pending</option>
          <option value="in_review">In review</option>
          <option value="decided">Decided</option>
        </select>
      </div>

      {loading && <p style={{ color: '#6b7280' }}>Loading…</p>}
      {error && <p style={{ color: '#dc2626' }}>{error}</p>}
      {result && result.items.length === 0 && !loading && (
        <p style={{ color: '#6b7280' }}>No items in queue.</p>
      )}

      {result && result.items.length > 0 && (
        <>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr style={{ background: '#f3f4f6', borderBottom: '1px solid #e5e7eb' }}>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Content ID</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Status</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Assigned to</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Decision</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Created</th>
                  <th style={{ padding: '0.5rem 0.75rem' }}></th>
                </tr>
              </thead>
              <tbody>
                {result.items.map((item) => (
                  <tr key={item.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                    <td style={{ padding: '0.5rem 0.75rem', fontFamily: 'monospace', fontSize: '0.75rem' }}>{item.content_id.slice(0, 8)}…</td>
                    <td style={{ padding: '0.5rem 0.75rem' }}><StatusBadge status={item.status} /></td>
                    <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280', fontSize: '0.75rem' }}>{item.assigned_to ? item.assigned_to.slice(0, 8) + '…' : '—'}</td>
                    <td style={{ padding: '0.5rem 0.75rem' }}>
                      {item.decision ? (
                        <span style={{ color: item.decision === 'approved' ? '#065f46' : '#991b1b', fontWeight: 600 }}>
                          {item.decision}
                        </span>
                      ) : '—'}
                    </td>
                    <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280', fontSize: '0.75rem' }}>
                      {new Date(item.created_at).toLocaleString()}
                    </td>
                    <td style={{ padding: '0.5rem 0.75rem' }}>
                      <button
                        onClick={() => navigate(`/moderation/${item.id}`)}
                        style={{ fontSize: '0.75rem', color: '#4f46e5', background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline' }}
                      >
                        Review
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {result.total_pages > 1 && (
            <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem', alignItems: 'center' }}>
              <button onClick={() => { setPage(page - 1); fetchQueue(statusFilter, page - 1); }} disabled={page <= 1} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>← Prev</button>
              <span style={{ fontSize: '0.8125rem' }}>Page {page} of {result.total_pages}</span>
              <button onClick={() => { setPage(page + 1); fetchQueue(statusFilter, page + 1); }} disabled={page >= result.total_pages} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>Next →</button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
