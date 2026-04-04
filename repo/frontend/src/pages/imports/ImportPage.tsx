// ImportPage — bulk import upload, preview, and commit flow.
//
// Flow:
//  1. User selects import type and uploads a CSV file.
//  2. Server parses and stages rows; returns a job with status preview_ready or failed.
//  3. Page shows a validation summary and a paginated preview of staged rows.
//  4. If no errors: user can commit (atomic, full rollback on any DB error).
//  5. If errors: user downloads the error CSV and fixes the file before retrying.
//  6. User can also explicitly rollback a preview_ready job.

import { useCallback, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { importsApi, ImportJob, ImportRow, PreviewResponse, ImportType, RowError } from '../../api/imports';
import type { PageResult } from '../../api/readers';

// ── Status badge ──────────────────────────────────────────────────────────────

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  preview_ready: { bg: '#d1fae5', color: '#065f46' },
  committed:     { bg: '#dbeafe', color: '#1e40af' },
  failed:        { bg: '#fee2e2', color: '#991b1b' },
  rolled_back:   { bg: '#fef3c7', color: '#92400e' },
  uploading:     { bg: '#f3f4f6', color: '#374151' },
  previewing:    { bg: '#f3f4f6', color: '#374151' },
};

function StatusBadge({ status }: { status: string }) {
  const s = STATUS_STYLE[status] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{
      display: 'inline-block',
      padding: '0.125rem 0.5rem',
      borderRadius: '9999px',
      fontSize: '0.6875rem',
      fontWeight: 600,
      background: s.bg,
      color: s.color,
    }}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export default function ImportPage() {
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('imports:write');

  const [importType, setImportType] = useState<ImportType>('readers');
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  const [job, setJob] = useState<ImportJob | null>(null);
  const [preview, setPreview] = useState<PageResult<ImportRow> | null>(null);
  const [previewPage, setPreviewPage] = useState(1);
  const [loadingPreview, setLoadingPreview] = useState(false);

  const [committing, setCommitting] = useState(false);
  const [commitError, setCommitError] = useState<string | null>(null);

  const fileRef = useRef<HTMLInputElement>(null);

  // ── Upload ──────────────────────────────────────────────────────────────────

  const handleUpload = useCallback(async () => {
    if (!file) return;
    setUploading(true);
    setUploadError(null);
    setJob(null);
    setPreview(null);
    setCommitError(null);
    try {
      const newJob = await importsApi.upload(importType, file);
      setJob(newJob);
      await loadPreview(newJob.id, 1);
    } catch (err: unknown) {
      setUploadError(err instanceof Error ? err.message : 'Upload failed');
    } finally {
      setUploading(false);
    }
  }, [file, importType]);

  // ── Preview load ────────────────────────────────────────────────────────────

  const loadPreview = useCallback(async (jobId: string, page: number) => {
    setLoadingPreview(true);
    try {
      const resp: PreviewResponse = await importsApi.getPreview(jobId, { page, per_page: 20 });
      setJob(resp.job);
      setPreview(resp.rows);
      setPreviewPage(page);
    } catch {
      // Ignore — job was already set from upload
    } finally {
      setLoadingPreview(false);
    }
  }, []);

  // ── Commit ──────────────────────────────────────────────────────────────────

  const handleCommit = useCallback(async () => {
    if (!job) return;
    setCommitting(true);
    setCommitError(null);
    try {
      const committed = await importsApi.commit(job.id);
      setJob(committed);
    } catch (err: unknown) {
      setCommitError(err instanceof Error ? err.message : 'Commit failed');
      // Reload job state after commit failure (it may have rolled back).
      try {
        const resp = await importsApi.getPreview(job.id, { page: previewPage, per_page: 20 });
        setJob(resp.job);
        setPreview(resp.rows);
      } catch {
        // ignore
      }
    } finally {
      setCommitting(false);
    }
  }, [job, previewPage]);

  // ── Rollback ────────────────────────────────────────────────────────────────

  const handleRollback = useCallback(async () => {
    if (!job) return;
    if (!window.confirm('Cancel this import? Staged rows will be discarded.')) return;
    try {
      const rolled = await importsApi.rollback(job.id);
      setJob(rolled);
    } catch (err: unknown) {
      setCommitError(err instanceof Error ? err.message : 'Rollback failed');
    }
  }, [job]);

  // ── Render ──────────────────────────────────────────────────────────────────

  const isFinal = job && (job.status === 'committed' || job.status === 'rolled_back');

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '1.5rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Bulk Import</h1>
        <button
          onClick={() => navigate('/imports/history')}
          style={{ fontSize: '0.8125rem', color: '#4f46e5', background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline' }}
        >
          View import history
        </button>
      </div>

      {/* Upload form — hidden once a job is in a final state */}
      {!isFinal && (
        <section style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1.25rem', marginBottom: '1.5rem' }}>
          <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '1rem' }}>1. Select file</h2>

          <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
            <div>
              <label style={{ fontSize: '0.8125rem', fontWeight: 500, display: 'block', marginBottom: 4 }}>Import type</label>
              <select
                value={importType}
                onChange={(e) => setImportType(e.target.value as ImportType)}
                disabled={uploading || !!job}
                style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem' }}
              >
                <option value="readers">Readers</option>
                <option value="holdings">Holdings</option>
              </select>
            </div>

            <div>
              <label style={{ fontSize: '0.8125rem', fontWeight: 500, display: 'block', marginBottom: 4 }}>CSV file</label>
              <input
                ref={fileRef}
                type="file"
                accept=".csv,text/csv"
                disabled={uploading || !!job}
                onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                style={{ fontSize: '0.875rem' }}
              />
            </div>
          </div>

          <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
            <button
              onClick={handleUpload}
              disabled={!file || uploading || !!job || !canWrite}
              style={{
                padding: '0.5rem 1rem', background: '#4f46e5', color: '#fff',
                border: 'none', borderRadius: 6, fontSize: '0.875rem',
                cursor: (!file || uploading || !!job || !canWrite) ? 'not-allowed' : 'pointer',
                opacity: (!file || uploading || !!job || !canWrite) ? 0.6 : 1,
              }}
            >
              {uploading ? 'Uploading…' : 'Upload & Validate'}
            </button>

            <a
              href={importsApi.templateCsvUrl(importType)}
              download
              style={{ fontSize: '0.8125rem', color: '#4f46e5', textDecoration: 'underline' }}
            >
              Download {importType} CSV template
            </a>
          </div>

          {uploadError && (
            <p style={{ color: '#dc2626', fontSize: '0.8125rem', marginTop: '0.75rem' }}>{uploadError}</p>
          )}
        </section>
      )}

      {/* Preview & validation summary */}
      {job && (
        <section style={{ marginBottom: '1.5rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '1rem' }}>
            <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, margin: 0 }}>2. Validation summary</h2>
            <StatusBadge status={job.status} />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '0.75rem', marginBottom: '1rem' }}>
            <StatCard label="File" value={job.file_name} />
            <StatCard label="Rows" value={job.row_count != null ? String(job.row_count) : '—'} />
            <StatCard
              label="Errors"
              value={String(job.error_count)}
              valueColor={job.error_count > 0 ? '#dc2626' : '#059669'}
            />
          </div>

          {job.error_count > 0 && (
            <div style={{ background: '#fef2f2', border: '1px solid #fecaca', borderRadius: 8, padding: '1rem', marginBottom: '1rem' }}>
              <p style={{ margin: '0 0 0.5rem', fontWeight: 600, color: '#991b1b', fontSize: '0.875rem' }}>
                {job.error_count} row(s) failed validation. Fix the file and re-upload, or download the error report.
              </p>
              <p style={{ margin: '0 0 0.75rem', fontSize: '0.8125rem', color: '#7f1d1d' }}>
                The import cannot be committed until all rows are valid.
              </p>
              <a
                href={importsApi.errorsCsvUrl(job.id)}
                download
                style={{ fontSize: '0.8125rem', color: '#b91c1c', fontWeight: 500, textDecoration: 'underline' }}
              >
                Download error report (CSV)
              </a>

              {/* Inline error table (first 10 errors) */}
              {Array.isArray(job.error_summary) && job.error_summary.length > 0 && (
                <div style={{ marginTop: '0.75rem', overflowX: 'auto' }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid #fecaca' }}>
                        <th style={{ textAlign: 'left', padding: '0.25rem 0.5rem', color: '#7f1d1d' }}>Row</th>
                        <th style={{ textAlign: 'left', padding: '0.25rem 0.5rem', color: '#7f1d1d' }}>Field</th>
                        <th style={{ textAlign: 'left', padding: '0.25rem 0.5rem', color: '#7f1d1d' }}>Message</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(job.error_summary as RowError[]).slice(0, 10).map((e, i) => (
                        <tr key={i} style={{ borderBottom: '1px solid #fee2e2' }}>
                          <td style={{ padding: '0.25rem 0.5rem', fontFamily: 'monospace' }}>{e.row}</td>
                          <td style={{ padding: '0.25rem 0.5rem', fontFamily: 'monospace' }}>{e.field}</td>
                          <td style={{ padding: '0.25rem 0.5rem' }}>{e.message}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {(job.error_summary as RowError[]).length > 10 && (
                    <p style={{ fontSize: '0.75rem', color: '#7f1d1d', marginTop: 4 }}>
                      … and {(job.error_summary as RowError[]).length - 10} more. Download the full error report.
                    </p>
                  )}
                </div>
              )}
            </div>
          )}

          {/* Commit / rollback actions */}
          {canWrite && !isFinal && (
            <div style={{ display: 'flex', gap: '0.75rem', marginBottom: '1rem' }}>
              <button
                onClick={handleCommit}
                disabled={job.status !== 'preview_ready' || job.error_count > 0 || committing}
                style={{
                  padding: '0.5rem 1.25rem', background: '#059669', color: '#fff',
                  border: 'none', borderRadius: 6, fontSize: '0.875rem',
                  cursor: (job.status !== 'preview_ready' || job.error_count > 0 || committing) ? 'not-allowed' : 'pointer',
                  opacity: (job.status !== 'preview_ready' || job.error_count > 0 || committing) ? 0.6 : 1,
                }}
              >
                {committing ? 'Committing…' : 'Commit import'}
              </button>
              <button
                onClick={handleRollback}
                disabled={job.status !== 'preview_ready'}
                style={{
                  padding: '0.5rem 1.25rem', background: '#fff', color: '#dc2626',
                  border: '1px solid #fca5a5', borderRadius: 6, fontSize: '0.875rem',
                  cursor: job.status !== 'preview_ready' ? 'not-allowed' : 'pointer',
                  opacity: job.status !== 'preview_ready' ? 0.6 : 1,
                }}
              >
                Cancel import
              </button>
            </div>
          )}

          {commitError && (
            <p style={{ color: '#dc2626', fontSize: '0.8125rem', marginBottom: '0.75rem' }}>{commitError}</p>
          )}

          {job.status === 'committed' && (
            <div style={{ background: '#f0fdf4', border: '1px solid #bbf7d0', borderRadius: 8, padding: '1rem', marginBottom: '1rem' }}>
              <p style={{ margin: 0, color: '#166534', fontWeight: 600, fontSize: '0.875rem' }}>
                Import committed successfully — {job.row_count} row(s) inserted.
              </p>
            </div>
          )}

          {job.status === 'rolled_back' && (
            <div style={{ background: '#fffbeb', border: '1px solid #fde68a', borderRadius: 8, padding: '1rem', marginBottom: '1rem' }}>
              <p style={{ margin: 0, color: '#92400e', fontWeight: 600, fontSize: '0.875rem' }}>
                Import rolled back. No rows were written to the database.
              </p>
            </div>
          )}

          {/* Row preview table */}
          {preview && preview.items.length > 0 && (
            <div>
              <h3 style={{ fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.5rem' }}>
                Row preview ({preview.total} total)
              </h3>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
                  <thead>
                    <tr style={{ background: '#f3f4f6', borderBottom: '1px solid #e5e7eb' }}>
                      <th style={{ textAlign: 'left', padding: '0.375rem 0.5rem', fontWeight: 600 }}>Row</th>
                      <th style={{ textAlign: 'left', padding: '0.375rem 0.5rem', fontWeight: 600 }}>Status</th>
                      <th style={{ textAlign: 'left', padding: '0.375rem 0.5rem', fontWeight: 600 }}>Error</th>
                      <th style={{ textAlign: 'left', padding: '0.375rem 0.5rem', fontWeight: 600 }}>Data (raw)</th>
                    </tr>
                  </thead>
                  <tbody>
                    {preview.items.map((row) => (
                      <tr key={row.id} style={{ borderBottom: '1px solid #f3f4f6', background: row.status === 'invalid' ? '#fff5f5' : undefined }}>
                        <td style={{ padding: '0.375rem 0.5rem', fontFamily: 'monospace' }}>{row.row_number}</td>
                        <td style={{ padding: '0.375rem 0.5rem' }}>
                          <StatusBadge status={row.status} />
                        </td>
                        <td style={{ padding: '0.375rem 0.5rem', color: '#dc2626', fontSize: '0.75rem', maxWidth: 200 }}>
                          {row.error_details ?? '—'}
                        </td>
                        <td style={{ padding: '0.375rem 0.5rem', fontFamily: 'monospace', fontSize: '0.75rem', maxWidth: 350, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {JSON.stringify(row.raw_data)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              {preview.total_pages > 1 && (
                <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.75rem', alignItems: 'center' }}>
                  <button
                    onClick={() => loadPreview(job.id, previewPage - 1)}
                    disabled={previewPage <= 1 || loadingPreview}
                    style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem', cursor: previewPage <= 1 ? 'not-allowed' : 'pointer' }}
                  >
                    ← Prev
                  </button>
                  <span style={{ fontSize: '0.8125rem' }}>
                    Page {previewPage} of {preview.total_pages}
                  </span>
                  <button
                    onClick={() => loadPreview(job.id, previewPage + 1)}
                    disabled={previewPage >= preview.total_pages || loadingPreview}
                    style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem', cursor: previewPage >= preview.total_pages ? 'not-allowed' : 'pointer' }}
                  >
                    Next →
                  </button>
                </div>
              )}
            </div>
          )}
        </section>
      )}
    </div>
  );
}

// ── Stat card ─────────────────────────────────────────────────────────────────

function StatCard({ label, value, valueColor }: { label: string; value: string; valueColor?: string }) {
  return (
    <div style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '0.75rem 1rem' }}>
      <p style={{ margin: 0, fontSize: '0.75rem', color: '#6b7280' }}>{label}</p>
      <p style={{ margin: 0, fontSize: '1.125rem', fontWeight: 700, color: valueColor ?? '#111827', fontFamily: 'monospace' }}>{value}</p>
    </div>
  );
}
