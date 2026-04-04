// FeedbackPage — view and moderate reader feedback.
// Lists feedback items; moderators can approve/reject/flag inline.
// Staff can submit new feedback on behalf of a reader.

import { useCallback, useEffect, useState } from 'react';
import { useAuth } from '../../auth/AuthContext';
import { feedbackApi, Feedback, FeedbackTag } from '../../api/feedback';
import type { PageResult } from '../../api/readers';

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  pending:  { bg: '#fef3c7', color: '#92400e' },
  approved: { bg: '#d1fae5', color: '#065f46' },
  rejected: { bg: '#fee2e2', color: '#991b1b' },
  flagged:  { bg: '#fce7f3', color: '#9d174d' },
};

function StatusBadge({ status }: { status: string }) {
  const s = STATUS_STYLE[status] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{ display: 'inline-block', padding: '0.125rem 0.5rem', borderRadius: '9999px', fontSize: '0.6875rem', fontWeight: 600, background: s.bg, color: s.color }}>
      {status}
    </span>
  );
}

function StarRating({ rating }: { rating: number }) {
  return (
    <span style={{ color: '#f59e0b', fontSize: '0.875rem' }}>
      {'★'.repeat(rating)}{'☆'.repeat(5 - rating)}
    </span>
  );
}

export default function FeedbackPage() {
  const { hasPermission } = useAuth();
  const canRead = hasPermission('feedback:read');
  const canModerate = hasPermission('feedback:moderate');

  const [result, setResult] = useState<PageResult<Feedback> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [page, setPage] = useState(1);

  // Tags
  const [availableTags, setAvailableTags] = useState<FeedbackTag[]>([]);

  // Submit form
  const [showSubmit, setShowSubmit] = useState(false);
  const [submitReaderID, setSubmitReaderID] = useState('');
  const [submitTargetType, setSubmitTargetType] = useState<'holding' | 'program'>('program');
  const [submitTargetID, setSubmitTargetID] = useState('');
  const [submitRating, setSubmitRating] = useState<number>(5);
  const [submitComment, setSubmitComment] = useState('');
  const [submitTags, setSubmitTags] = useState<string[]>([]);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Moderate
  const [moderatingId, setModeratingId] = useState<string | null>(null);

  const fetchFeedback = useCallback(async (status: string, type: string, p: number) => {
    setLoading(true);
    setError(null);
    try {
      const data = await feedbackApi.list({
        status: status || undefined,
        target_type: type || undefined,
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
    if (canRead) {
      fetchFeedback('', '', 1);
      feedbackApi.listTags().then(setAvailableTags).catch(() => {});
    } else {
      setLoading(false);
    }
  }, [canRead]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitError(null);
    setSubmitting(true);
    try {
      await feedbackApi.submit({
        reader_id: submitReaderID.trim(),
        target_type: submitTargetType,
        target_id: submitTargetID.trim(),
        rating: submitRating,
        comment: submitComment.trim() || undefined,
        tags: submitTags,
      });
      setShowSubmit(false);
      setSubmitReaderID('');
      setSubmitTargetID('');
      setSubmitComment('');
      setSubmitTags([]);
      fetchFeedback(statusFilter, typeFilter, 1);
    } catch (err: unknown) {
      setSubmitError(err instanceof Error ? err.message : 'Submit failed');
    } finally {
      setSubmitting(false);
    }
  };

  const handleModerate = async (id: string, status: 'approved' | 'rejected' | 'flagged') => {
    try {
      await feedbackApi.moderate(id, status);
      setModeratingId(null);
      fetchFeedback(statusFilter, typeFilter, page);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Moderate failed');
    }
  };

  if (!canRead) {
    return <p style={{ padding: '1.5rem', color: '#dc2626' }}>You do not have permission to view feedback.</p>;
  }

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: '1.5rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Feedback</h1>
        <button onClick={() => setShowSubmit(!showSubmit)}
          style={{ padding: '0.5rem 1rem', background: '#4f46e5', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}>
          + Submit feedback
        </button>
      </div>

      {/* Submit form */}
      {showSubmit && (
        <form onSubmit={handleSubmit} style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1.25rem', marginBottom: '1.5rem' }}>
          <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.75rem' }}>Submit feedback for a reader</h2>
          {submitError && <p style={{ color: '#dc2626', fontSize: '0.875rem', marginBottom: '0.5rem' }}>{submitError}</p>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '0.75rem' }}>
            <div>
              <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Reader ID *</label>
              <input value={submitReaderID} onChange={(e) => setSubmitReaderID(e.target.value)}
                style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box', fontFamily: 'monospace' }} required />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Target type *</label>
              <select value={submitTargetType} onChange={(e) => setSubmitTargetType(e.target.value as 'holding' | 'program')}
                style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box' }}>
                <option value="program">Program</option>
                <option value="holding">Holding</option>
              </select>
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Target ID *</label>
              <input value={submitTargetID} onChange={(e) => setSubmitTargetID(e.target.value)}
                style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box', fontFamily: 'monospace' }} required />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Rating (1–5)</label>
              <input type="number" min={1} max={5} value={submitRating} onChange={(e) => setSubmitRating(parseInt(e.target.value))}
                style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box' }} />
            </div>
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Comment</label>
            <textarea value={submitComment} onChange={(e) => setSubmitComment(e.target.value)}
              style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: '100%', boxSizing: 'border-box', minHeight: 64, resize: 'vertical' }} />
          </div>
          {availableTags.length > 0 && (
            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.375rem' }}>Tags</label>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.375rem' }}>
                {availableTags.map((tag) => (
                  <label key={tag.id} style={{ display: 'flex', gap: '0.25rem', alignItems: 'center', cursor: 'pointer', fontSize: '0.8125rem' }}>
                    <input type="checkbox" checked={submitTags.includes(tag.name)} onChange={(e) => {
                      if (e.target.checked) setSubmitTags([...submitTags, tag.name]);
                      else setSubmitTags(submitTags.filter(t => t !== tag.name));
                    }} />
                    {tag.name}
                  </label>
                ))}
              </div>
            </div>
          )}
          <div style={{ display: 'flex', gap: '0.75rem' }}>
            <button type="submit" disabled={submitting}
              style={{ padding: '0.375rem 0.875rem', background: '#059669', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}>
              {submitting ? 'Submitting…' : 'Submit'}
            </button>
            <button type="button" onClick={() => setShowSubmit(false)}
              style={{ padding: '0.375rem 0.75rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}>
              Cancel
            </button>
          </div>
        </form>
      )}

      {/* Filters */}
      <div style={{ display: 'flex', gap: '0.75rem', marginBottom: '1rem' }}>
        <select value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value); setPage(1); fetchFeedback(e.target.value, typeFilter, 1); }}
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem' }}>
          <option value="">All statuses</option>
          <option value="pending">Pending</option>
          <option value="approved">Approved</option>
          <option value="rejected">Rejected</option>
          <option value="flagged">Flagged</option>
        </select>
        <select value={typeFilter} onChange={(e) => { setTypeFilter(e.target.value); setPage(1); fetchFeedback(statusFilter, e.target.value, 1); }}
          style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem' }}>
          <option value="">All targets</option>
          <option value="program">Program</option>
          <option value="holding">Holding</option>
        </select>
      </div>

      {loading && <p style={{ color: '#6b7280' }}>Loading…</p>}
      {error && <p style={{ color: '#dc2626' }}>{error}</p>}
      {result && result.items.length === 0 && !loading && <p style={{ color: '#6b7280' }}>No feedback found.</p>}

      {result && result.items.length > 0 && (
        <>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            {result.items.map((fb) => (
              <div key={fb.id} style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1rem' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                  <div>
                    <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', marginBottom: '0.25rem' }}>
                      <StatusBadge status={fb.status} />
                      <span style={{ fontSize: '0.75rem', color: '#6b7280' }}>{fb.target_type} · {fb.target_id.slice(0, 8)}…</span>
                    </div>
                    {fb.rating != null && <StarRating rating={fb.rating} />}
                    {fb.comment && <p style={{ margin: '0.25rem 0 0', fontSize: '0.875rem', color: '#374151' }}>{fb.comment}</p>}
                    {fb.tags.length > 0 && (
                      <div style={{ display: 'flex', gap: '0.25rem', flexWrap: 'wrap', marginTop: '0.375rem' }}>
                        {fb.tags.map(tag => (
                          <span key={tag} style={{ fontSize: '0.6875rem', background: '#ede9fe', color: '#5b21b6', borderRadius: '9999px', padding: '0.125rem 0.375rem' }}>{tag}</span>
                        ))}
                      </div>
                    )}
                    <p style={{ margin: '0.25rem 0 0', fontSize: '0.75rem', color: '#9ca3af' }}>
                      Reader {fb.reader_id.slice(0, 8)}… · {new Date(fb.submitted_at).toLocaleString()}
                    </p>
                  </div>

                  {/* Moderation actions */}
                  {canModerate && fb.status === 'pending' && (
                    <div style={{ display: 'flex', gap: '0.375rem' }}>
                      {moderatingId === fb.id ? (
                        <>
                          <button onClick={() => handleModerate(fb.id, 'approved')} style={{ fontSize: '0.75rem', color: '#065f46', background: '#d1fae5', border: 'none', borderRadius: 4, padding: '0.25rem 0.5rem', cursor: 'pointer' }}>Approve</button>
                          <button onClick={() => handleModerate(fb.id, 'rejected')} style={{ fontSize: '0.75rem', color: '#991b1b', background: '#fee2e2', border: 'none', borderRadius: 4, padding: '0.25rem 0.5rem', cursor: 'pointer' }}>Reject</button>
                          <button onClick={() => handleModerate(fb.id, 'flagged')} style={{ fontSize: '0.75rem', color: '#9d174d', background: '#fce7f3', border: 'none', borderRadius: 4, padding: '0.25rem 0.5rem', cursor: 'pointer' }}>Flag</button>
                          <button onClick={() => setModeratingId(null)} style={{ fontSize: '0.75rem', color: '#6b7280', background: 'none', border: 'none', cursor: 'pointer' }}>Cancel</button>
                        </>
                      ) : (
                        <button onClick={() => setModeratingId(fb.id)} style={{ fontSize: '0.75rem', color: '#4f46e5', background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline' }}>Moderate</button>
                      )}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>

          {result.total_pages > 1 && (
            <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1rem', alignItems: 'center' }}>
              <button onClick={() => { setPage(page - 1); fetchFeedback(statusFilter, typeFilter, page - 1); }} disabled={page <= 1} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>← Prev</button>
              <span style={{ fontSize: '0.8125rem' }}>Page {page} of {result.total_pages}</span>
              <button onClick={() => { setPage(page + 1); fetchFeedback(statusFilter, typeFilter, page + 1); }} disabled={page >= result.total_pages} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>Next →</button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
