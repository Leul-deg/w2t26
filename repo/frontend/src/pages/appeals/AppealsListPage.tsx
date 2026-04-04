// AppealsListPage — list appeals with status/type/reader filters.
// Requires appeals:read permission. Any authenticated user can submit.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { appealsApi, Appeal, AppealType } from '../../api/appeals';
import type { PageResult } from '../../api/readers';

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  submitted:    { bg: '#fef3c7', color: '#92400e' },
  under_review: { bg: '#dbeafe', color: '#1e40af' },
  resolved:     { bg: '#d1fae5', color: '#065f46' },
  dismissed:    { bg: '#fee2e2', color: '#991b1b' },
};

function StatusBadge({ status }: { status: string }) {
  const s = STATUS_STYLE[status] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{ display: 'inline-block', padding: '0.125rem 0.5rem', borderRadius: '9999px', fontSize: '0.6875rem', fontWeight: 600, background: s.bg, color: s.color }}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

const APPEAL_TYPE_LABELS: Record<string, string> = {
  enrollment_denial: 'Enrollment denial',
  account_suspension: 'Account suspension',
  feedback_rejection: 'Feedback rejection',
  blacklist_removal: 'Blacklist removal',
  other: 'Other',
};

export default function AppealsListPage() {
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canRead = hasPermission('appeals:read');

  const [result, setResult] = useState<PageResult<Appeal> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [readerFilter, setReaderFilter] = useState('');
  const [page, setPage] = useState(1);

  // Submit form
  const [showSubmit, setShowSubmit] = useState(false);
  const [submitReaderID, setSubmitReaderID] = useState('');
  const [submitType, setSubmitType] = useState<AppealType>('other');
  const [submitReason, setSubmitReason] = useState('');
  const [submitTargetType, setSubmitTargetType] = useState('');
  const [submitTargetID, setSubmitTargetID] = useState('');
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const fetchAppeals = useCallback(async (status: string, type: string, reader: string, p: number) => {
    setLoading(true);
    setError(null);
    try {
      const data = await appealsApi.list({
        status: status || undefined,
        appeal_type: type || undefined,
        reader_id: reader.trim() || undefined,
        page: p,
        per_page: 20,
      });
      setResult(data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (canRead) fetchAppeals('', '', '', 1);
    else setLoading(false);
  }, [canRead]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitError(null);
    setSubmitting(true);
    try {
      const created = await appealsApi.submit({
        reader_id: submitReaderID.trim(),
        appeal_type: submitType,
        reason: submitReason.trim(),
        target_type: submitTargetType.trim() || undefined,
        target_id: submitTargetID.trim() || undefined,
      });
      setShowSubmit(false);
      setSubmitReaderID('');
      setSubmitReason('');
      setSubmitTargetType('');
      setSubmitTargetID('');
      navigate(`/appeals/${created.id}`);
    } catch (err: unknown) {
      setSubmitError(err instanceof Error ? err.message : 'Submit failed');
    } finally {
      setSubmitting(false);
    }
  };

  if (!canRead && !showSubmit) {
    return (
      <div style={{ maxWidth: 960, margin: '0 auto', padding: '1.5rem' }}>
        <p style={{ color: '#dc2626' }}>You do not have permission to view appeals.</p>
        <button onClick={() => setShowSubmit(true)}
          style={{ padding: '0.5rem 1rem', background: '#4f46e5', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer', marginTop: '0.75rem' }}>
          Submit an appeal
        </button>
        {showSubmit && renderSubmitForm()}
      </div>
    );
  }

  function renderSubmitForm() {
    return (
      <form onSubmit={handleSubmit} style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1.25rem', marginTop: '1rem' }}>
        <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.75rem' }}>Submit an appeal</h2>
        {submitError && <p style={{ color: '#dc2626', fontSize: '0.875rem', marginBottom: '0.5rem' }}>{submitError}</p>}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '0.75rem' }}>
          <div>
            <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Reader ID *</label>
            <input value={submitReaderID} onChange={(e) => setSubmitReaderID(e.target.value)}
              style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box', fontFamily: 'monospace' }} required />
          </div>
          <div>
            <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Appeal type *</label>
            <select value={submitType} onChange={(e) => setSubmitType(e.target.value as AppealType)}
              style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box' }}>
              {Object.entries(APPEAL_TYPE_LABELS).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
            </select>
          </div>
          <div>
            <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Target type</label>
            <input value={submitTargetType} onChange={(e) => setSubmitTargetType(e.target.value)} placeholder="e.g. program"
              style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box' }} />
          </div>
          <div>
            <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Target ID</label>
            <input value={submitTargetID} onChange={(e) => setSubmitTargetID(e.target.value)}
              style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box', fontFamily: 'monospace' }} />
          </div>
        </div>
        <div style={{ marginBottom: '0.75rem' }}>
          <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Reason *</label>
          <textarea value={submitReason} onChange={(e) => setSubmitReason(e.target.value)} required
            style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box', minHeight: 80, resize: 'vertical' }} />
        </div>
        <div style={{ display: 'flex', gap: '0.75rem' }}>
          <button type="submit" disabled={submitting}
            style={{ padding: '0.375rem 0.875rem', background: '#059669', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}>
            {submitting ? 'Submitting…' : 'Submit appeal'}
          </button>
          <button type="button" onClick={() => setShowSubmit(false)}
            style={{ padding: '0.375rem 0.75rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}>
            Cancel
          </button>
        </div>
      </form>
    );
  }

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: '1.5rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Appeals</h1>
        <button onClick={() => setShowSubmit(!showSubmit)}
          style={{ padding: '0.5rem 1rem', background: '#4f46e5', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}>
          + Submit appeal
        </button>
      </div>

      {showSubmit && renderSubmitForm()}

      {/* Filters */}
      <div style={{ display: 'flex', gap: '0.75rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
        <select value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value); setPage(1); fetchAppeals(e.target.value, typeFilter, readerFilter, 1); }}
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem' }}>
          <option value="">All statuses</option>
          <option value="submitted">Submitted</option>
          <option value="under_review">Under review</option>
          <option value="resolved">Resolved</option>
          <option value="dismissed">Dismissed</option>
        </select>
        <select value={typeFilter} onChange={(e) => { setTypeFilter(e.target.value); setPage(1); fetchAppeals(statusFilter, e.target.value, readerFilter, 1); }}
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem' }}>
          <option value="">All types</option>
          {Object.entries(APPEAL_TYPE_LABELS).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
        </select>
        <input
          value={readerFilter}
          onChange={(e) => setReaderFilter(e.target.value)}
          onBlur={() => { setPage(1); fetchAppeals(statusFilter, typeFilter, readerFilter, 1); }}
          placeholder="Filter by reader ID…"
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', fontFamily: 'monospace', width: 200 }}
        />
      </div>

      {loading && <p style={{ color: '#6b7280' }}>Loading…</p>}
      {error && <p style={{ color: '#dc2626' }}>{error}</p>}
      {result && result.items.length === 0 && !loading && <p style={{ color: '#6b7280' }}>No appeals found.</p>}

      {result && result.items.length > 0 && (
        <>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr style={{ background: '#f3f4f6', borderBottom: '1px solid #e5e7eb' }}>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Reader</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Type</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Status</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Target</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Submitted</th>
                  <th style={{ padding: '0.5rem 0.75rem' }}></th>
                </tr>
              </thead>
              <tbody>
                {result.items.map((appeal) => (
                  <tr key={appeal.id} style={{ borderBottom: '1px solid #f3f4f6', cursor: 'pointer' }} onClick={() => navigate(`/appeals/${appeal.id}`)}>
                    <td style={{ padding: '0.5rem 0.75rem', fontFamily: 'monospace', fontSize: '0.75rem' }}>{appeal.reader_id.slice(0, 8)}…</td>
                    <td style={{ padding: '0.5rem 0.75rem', color: '#374151' }}>{APPEAL_TYPE_LABELS[appeal.appeal_type] ?? appeal.appeal_type}</td>
                    <td style={{ padding: '0.5rem 0.75rem' }}><StatusBadge status={appeal.status} /></td>
                    <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280', fontSize: '0.75rem' }}>
                      {appeal.target_type ? `${appeal.target_type} · ${(appeal.target_id ?? '').slice(0, 8)}…` : '—'}
                    </td>
                    <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280', fontSize: '0.75rem' }}>{new Date(appeal.submitted_at).toLocaleString()}</td>
                    <td style={{ padding: '0.5rem 0.75rem' }}>
                      <button onClick={(e) => { e.stopPropagation(); navigate(`/appeals/${appeal.id}`); }}
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
              <button onClick={() => { setPage(page - 1); fetchAppeals(statusFilter, typeFilter, readerFilter, page - 1); }} disabled={page <= 1} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>← Prev</button>
              <span style={{ fontSize: '0.8125rem' }}>Page {page} of {result.total_pages}</span>
              <button onClick={() => { setPage(page + 1); fetchAppeals(statusFilter, typeFilter, readerFilter, page + 1); }} disabled={page >= result.total_pages} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>Next →</button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
