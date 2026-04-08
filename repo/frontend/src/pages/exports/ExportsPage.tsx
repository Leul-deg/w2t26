// ExportsPage — trigger exports and view the audit log of past exports.
//
// Export actions are gated by exports:create permission.
// Every export generates a CSV download and an audit record.
// The page shows the audit history so reviewers can trace every export.

import { useCallback, useEffect, useState } from 'react';
import { useAuth } from '../../auth/AuthContext';
import { exportsApi, ExportJob } from '../../api/exports';
import type { PageResult } from '../../api/readers';

export default function ExportsPage() {
  const { hasPermission } = useAuth();
  const canExport = hasPermission('exports:create');
  const canRead = canExport;

  const [exporting, setExporting] = useState<string | null>(null); // current export type/format
  const [exportError, setExportError] = useState<string | null>(null);

  const [history, setHistory] = useState<PageResult<ExportJob> | null>(null);
  const [historyPage, setHistoryPage] = useState(1);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // ── Fetch history ────────────────────────────────────────────────────────────

  const fetchHistory = useCallback(async (page: number) => {
    if (!canRead) return;
    setLoadingHistory(true);
    try {
      const data = await exportsApi.list({ page, per_page: 20 });
      setHistory(data);
      setHistoryPage(page);
    } catch {
      // non-fatal
    } finally {
      setLoadingHistory(false);
    }
  }, [canRead]);

  useEffect(() => { fetchHistory(1); }, [fetchHistory]);

  // ── Trigger export ────────────────────────────────────────────────────────────

  const handleExport = useCallback(async (exportType: 'readers' | 'holdings', format: 'csv' | 'xlsx') => {
    setExporting(`${exportType}:${format}`);
    setExportError(null);
    try {
      const { blob, fileName } = await exportsApi.triggerExport(exportType, format);
      // Trigger browser download.
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = fileName;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      // Refresh the audit history.
      await fetchHistory(1);
    } catch (err: unknown) {
      setExportError(err instanceof Error ? err.message : 'Export failed');
    } finally {
      setExporting(null);
    }
  }, [fetchHistory]);

  // ── Render ────────────────────────────────────────────────────────────────────

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '1.5rem' }}>
      <h1 style={{ fontSize: '1.25rem', fontWeight: 700, marginBottom: '1.5rem' }}>Data Export</h1>

      {/* Export actions */}
      {canExport && (
        <section style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1.25rem', marginBottom: '1.5rem' }}>
          <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.75rem' }}>Generate export</h2>
          <p style={{ fontSize: '0.8125rem', color: '#6b7280', marginBottom: '1rem' }}>
            Every export is audit-logged. Sensitive fields (national ID, contact details) are not included.
          </p>
          <div style={{ display: 'flex', gap: '0.75rem', flexWrap: 'wrap' }}>
            <ExportButton
              label="Export Readers (CSV)"
              onClick={() => handleExport('readers', 'csv')}
              loading={exporting === 'readers:csv'}
              disabled={!!exporting}
            />
            <ExportButton
              label="Export Readers (Excel)"
              onClick={() => handleExport('readers', 'xlsx')}
              loading={exporting === 'readers:xlsx'}
              disabled={!!exporting}
            />
            <ExportButton
              label="Export Holdings (CSV)"
              onClick={() => handleExport('holdings', 'csv')}
              loading={exporting === 'holdings:csv'}
              disabled={!!exporting}
            />
            <ExportButton
              label="Export Holdings (Excel)"
              onClick={() => handleExport('holdings', 'xlsx')}
              loading={exporting === 'holdings:xlsx'}
              disabled={!!exporting}
            />
          </div>
          {exportError && (
            <p style={{ color: '#dc2626', fontSize: '0.8125rem', marginTop: '0.75rem' }}>{exportError}</p>
          )}
        </section>
      )}

      {!canExport && (
        <div style={{ background: '#fef3c7', border: '1px solid #fde68a', borderRadius: 8, padding: '1rem', marginBottom: '1.5rem' }}>
          <p style={{ margin: 0, fontSize: '0.875rem', color: '#92400e' }}>
            You do not have permission to export data. Contact an administrator.
          </p>
        </div>
      )}

      {/* Export audit history */}
      {canRead && (
        <section>
          <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.75rem' }}>Export audit log</h2>

          {loadingHistory && <p style={{ color: '#6b7280', fontSize: '0.875rem' }}>Loading…</p>}

          {history && history.items.length === 0 && !loadingHistory && (
            <p style={{ color: '#6b7280', fontSize: '0.875rem' }}>No exports recorded yet.</p>
          )}

          {history && history.items.length > 0 && (
            <>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
                  <thead>
                    <tr style={{ background: '#f3f4f6', borderBottom: '1px solid #e5e7eb' }}>
                      <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Type</th>
                      <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>File</th>
                      <th style={{ textAlign: 'right', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Rows</th>
                      <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Exported by</th>
                      <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Exported at</th>
                    </tr>
                  </thead>
                  <tbody>
                    {history.items.map((job) => (
                      <tr key={job.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                        <td style={{ padding: '0.5rem 0.75rem' }}>
                          <span style={{
                            display: 'inline-block', padding: '0.125rem 0.5rem', borderRadius: '9999px',
                            fontSize: '0.6875rem', fontWeight: 600, background: '#ede9fe', color: '#5b21b6',
                          }}>
                            {job.export_type}
                          </span>
                        </td>
                        <td style={{ padding: '0.5rem 0.75rem', fontFamily: 'monospace', fontSize: '0.75rem', color: '#374151' }}>
                          {job.file_name ?? '—'}
                        </td>
                        <td style={{ padding: '0.5rem 0.75rem', textAlign: 'right', fontFamily: 'monospace' }}>
                          {job.row_count ?? '—'}
                        </td>
                        <td style={{ padding: '0.5rem 0.75rem', fontFamily: 'monospace', fontSize: '0.75rem' }}>
                          {job.exported_by}
                        </td>
                        <td style={{ padding: '0.5rem 0.75rem', fontSize: '0.75rem', color: '#6b7280' }}>
                          {new Date(job.exported_at).toLocaleString()}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {history.total_pages > 1 && (
                <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem', alignItems: 'center' }}>
                  <button
                    onClick={() => fetchHistory(historyPage - 1)}
                    disabled={historyPage <= 1 || loadingHistory}
                    style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}
                  >
                    ← Prev
                  </button>
                  <span style={{ fontSize: '0.8125rem' }}>
                    Page {historyPage} of {history.total_pages}
                  </span>
                  <button
                    onClick={() => fetchHistory(historyPage + 1)}
                    disabled={historyPage >= history.total_pages || loadingHistory}
                    style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}
                  >
                    Next →
                  </button>
                </div>
              )}
            </>
          )}
        </section>
      )}
    </div>
  );
}

// ── Export button ─────────────────────────────────────────────────────────────

function ExportButton({
  label, onClick, loading, disabled,
}: {
  label: string;
  onClick: () => void;
  loading: boolean;
  disabled: boolean;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      style={{
        padding: '0.5rem 1rem', background: '#4f46e5', color: '#fff',
        border: 'none', borderRadius: 6, fontSize: '0.875rem',
        cursor: disabled ? 'not-allowed' : 'pointer',
        opacity: disabled ? 0.6 : 1,
        minWidth: 140,
      }}
    >
      {loading ? 'Exporting…' : label}
    </button>
  );
}
