// ImportHistoryPage — paginated list of all import jobs for the current branch.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { importsApi, ImportJob } from '../../api/imports';
import type { PageResult } from '../../api/readers';

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  preview_ready: { bg: '#d1fae5', color: '#065f46' },
  committed:     { bg: '#dbeafe', color: '#1e40af' },
  failed:        { bg: '#fee2e2', color: '#991b1b' },
  rolled_back:   { bg: '#fef3c7', color: '#92400e' },
  previewing:    { bg: '#f3f4f6', color: '#374151' },
  uploaded:      { bg: '#f3f4f6', color: '#374151' },
};

function StatusBadge({ status }: { status: string }) {
  const s = STATUS_STYLE[status] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{
      display: 'inline-block', padding: '0.125rem 0.5rem', borderRadius: '9999px',
      fontSize: '0.6875rem', fontWeight: 600, background: s.bg, color: s.color,
    }}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

export default function ImportHistoryPage() {
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canRead = hasPermission('imports:read') || hasPermission('imports:write');

  const [result, setResult] = useState<PageResult<ImportJob> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  const fetchJobs = useCallback(async (p: number) => {
    setLoading(true);
    setError(null);
    try {
      const data = await importsApi.list({ page: p, per_page: 20 });
      setResult(data);
      setPage(p);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load import history');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchJobs(1); }, [fetchJobs]);

  if (!canRead) {
    return <p style={{ padding: '1.5rem', color: '#6b7280' }}>You do not have permission to view import history.</p>;
  }

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '1.5rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Import History</h1>
        {hasPermission('imports:write') && (
          <button
            onClick={() => navigate('/imports')}
            style={{
              padding: '0.5rem 1rem', background: '#4f46e5', color: '#fff',
              border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer',
            }}
          >
            New import
          </button>
        )}
      </div>

      {loading && <p style={{ color: '#6b7280' }}>Loading…</p>}
      {error && <p style={{ color: '#dc2626' }}>{error}</p>}

      {result && result.items.length === 0 && !loading && (
        <p style={{ color: '#6b7280' }}>No imports yet.</p>
      )}

      {result && result.items.length > 0 && (
        <>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr style={{ background: '#f3f4f6', borderBottom: '1px solid #e5e7eb' }}>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>File</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Type</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Status</th>
                  <th style={{ textAlign: 'right', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Rows</th>
                  <th style={{ textAlign: 'right', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Errors</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Uploaded</th>
                  <th style={{ padding: '0.5rem 0.75rem' }}></th>
                </tr>
              </thead>
              <tbody>
                {result.items.map((job) => (
                  <tr key={job.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                    <td style={{ padding: '0.5rem 0.75rem', fontFamily: 'monospace', fontSize: '0.75rem' }}>
                      {job.file_name}
                    </td>
                    <td style={{ padding: '0.5rem 0.75rem' }}>{job.import_type}</td>
                    <td style={{ padding: '0.5rem 0.75rem' }}><StatusBadge status={job.status} /></td>
                    <td style={{ padding: '0.5rem 0.75rem', textAlign: 'right', fontFamily: 'monospace' }}>
                      {job.row_count ?? '—'}
                    </td>
                    <td style={{ padding: '0.5rem 0.75rem', textAlign: 'right', fontFamily: 'monospace',
                                 color: job.error_count > 0 ? '#dc2626' : undefined }}>
                      {job.error_count}
                    </td>
                    <td style={{ padding: '0.5rem 0.75rem', fontSize: '0.75rem', color: '#6b7280' }}>
                      {new Date(job.uploaded_at).toLocaleString()}
                    </td>
                    <td style={{ padding: '0.5rem 0.75rem' }}>
                      {job.error_count > 0 && (
                        <a
                          href={importsApi.errorsCsvUrl(job.id)}
                          download
                          style={{ fontSize: '0.75rem', color: '#b91c1c', textDecoration: 'underline' }}
                        >
                          Errors
                        </a>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {result.total_pages > 1 && (
            <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem', alignItems: 'center' }}>
              <button
                onClick={() => fetchJobs(page - 1)}
                disabled={page <= 1}
                style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem', cursor: page <= 1 ? 'not-allowed' : 'pointer' }}
              >
                ← Prev
              </button>
              <span style={{ fontSize: '0.8125rem' }}>
                Page {page} of {result.total_pages}
              </span>
              <button
                onClick={() => fetchJobs(page + 1)}
                disabled={page >= result.total_pages}
                style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem', cursor: page >= result.total_pages ? 'not-allowed' : 'pointer' }}
              >
                Next →
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
