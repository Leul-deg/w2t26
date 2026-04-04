// ContentListPage — list of governed content items.
// Requires content:read permission. Authors with content:submit can create new items.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { contentApi, GovernedContent, ContentType } from '../../api/content';
import type { PageResult } from '../../api/readers';

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  draft:          { bg: '#f3f4f6', color: '#374151' },
  pending_review: { bg: '#fef3c7', color: '#92400e' },
  approved:       { bg: '#d1fae5', color: '#065f46' },
  rejected:       { bg: '#fee2e2', color: '#991b1b' },
  published:      { bg: '#dbeafe', color: '#1e40af' },
  archived:       { bg: '#e5e7eb', color: '#4b5563' },
};

function StatusBadge({ status }: { status: string }) {
  const s = STATUS_STYLE[status] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{ display: 'inline-block', padding: '0.125rem 0.5rem', borderRadius: '9999px', fontSize: '0.6875rem', fontWeight: 600, background: s.bg, color: s.color }}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

export default function ContentListPage() {
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canRead = hasPermission('content:read');
  const canSubmit = hasPermission('content:submit');

  const [result, setResult] = useState<PageResult<GovernedContent> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [page, setPage] = useState(1);

  const fetch = useCallback(async (status: string, type: string, p: number) => {
    setLoading(true);
    setError(null);
    try {
      const data = await contentApi.list({
        status: status || undefined,
        content_type: (type || undefined) as ContentType | undefined,
        page: p,
        per_page: 20,
      });
      setResult(data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load content');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { if (canRead) fetch('', '', 1); }, [canRead]);

  if (!canRead) {
    return <p style={{ padding: '1.5rem', color: '#dc2626' }}>You do not have permission to view content.</p>;
  }

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: '1.5rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Content</h1>
        {canSubmit && (
          <button
            onClick={() => navigate('/content/new')}
            style={{ padding: '0.5rem 1rem', background: '#4f46e5', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}
          >
            + New content
          </button>
        )}
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: '0.75rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setPage(1); fetch(e.target.value, typeFilter, 1); }}
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem' }}
        >
          <option value="">All statuses</option>
          <option value="draft">Draft</option>
          <option value="pending_review">Pending review</option>
          <option value="approved">Approved</option>
          <option value="rejected">Rejected</option>
          <option value="published">Published</option>
          <option value="archived">Archived</option>
        </select>
        <select
          value={typeFilter}
          onChange={(e) => { setTypeFilter(e.target.value); setPage(1); fetch(statusFilter, e.target.value, 1); }}
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem' }}
        >
          <option value="">All types</option>
          <option value="announcement">Announcement</option>
          <option value="document">Document</option>
          <option value="digital_resource">Digital resource</option>
          <option value="policy">Policy</option>
        </select>
      </div>

      {loading && <p style={{ color: '#6b7280' }}>Loading…</p>}
      {error && <p style={{ color: '#dc2626' }}>{error}</p>}
      {result && result.items.length === 0 && !loading && (
        <p style={{ color: '#6b7280' }}>No content items found.</p>
      )}

      {result && result.items.length > 0 && (
        <>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr style={{ background: '#f3f4f6', borderBottom: '1px solid #e5e7eb' }}>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Title</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Type</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Status</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Created</th>
                  <th style={{ padding: '0.5rem 0.75rem' }}></th>
                </tr>
              </thead>
              <tbody>
                {result.items.map((item) => (
                  <tr key={item.id} style={{ borderBottom: '1px solid #f3f4f6', cursor: 'pointer' }} onClick={() => navigate(`/content/${item.id}`)}>
                    <td style={{ padding: '0.5rem 0.75rem', fontWeight: 500 }}>{item.title}</td>
                    <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280' }}>{item.content_type.replace(/_/g, ' ')}</td>
                    <td style={{ padding: '0.5rem 0.75rem' }}><StatusBadge status={item.status} /></td>
                    <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280', fontSize: '0.75rem' }}>{new Date(item.created_at).toLocaleDateString()}</td>
                    <td style={{ padding: '0.5rem 0.75rem' }}>
                      <button onClick={(e) => { e.stopPropagation(); navigate(`/content/${item.id}`); }}
                        style={{ fontSize: '0.75rem', color: '#4f46e5', background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline' }}>
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
              <button onClick={() => { setPage(page - 1); fetch(statusFilter, typeFilter, page - 1); }} disabled={page <= 1} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>← Prev</button>
              <span style={{ fontSize: '0.8125rem' }}>Page {page} of {result.total_pages}</span>
              <button onClick={() => { setPage(page + 1); fetch(statusFilter, typeFilter, page + 1); }} disabled={page >= result.total_pages} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>Next →</button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
