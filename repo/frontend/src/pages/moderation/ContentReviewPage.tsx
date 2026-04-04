// ContentReviewPage — detail view for a moderation queue item.
// Shows content, allows assign-to-self and approve/reject decision.
// Requires content:moderate permission.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { moderationApi, ModerationItemWithContent } from '../../api/moderation';

export default function ContentReviewPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canModerate = hasPermission('content:moderate');

  const [data, setData] = useState<ModerationItemWithContent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [decision, setDecision] = useState<'approved' | 'rejected'>('approved');
  const [reason, setReason] = useState('');
  const [deciding, setDeciding] = useState(false);
  const [decideError, setDecideError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const d = await moderationApi.getItem(id);
      setData(d);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load item');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const handleAssign = async () => {
    if (!id) return;
    await moderationApi.assignItem(id);
    load();
  };

  const handleDecide = async () => {
    if (!id) return;
    setDecideError(null);
    setDeciding(true);
    try {
      await moderationApi.decideItem(id, decision, reason);
      navigate('/moderation');
    } catch (err: unknown) {
      setDecideError(err instanceof Error ? err.message : 'Decision failed');
    } finally {
      setDeciding(false);
    }
  };

  if (!canModerate) return <p style={{ padding: '1.5rem', color: '#dc2626' }}>Permission denied.</p>;
  if (loading) return <p style={{ padding: '1.5rem', color: '#6b7280' }}>Loading…</p>;
  if (error) return <p style={{ padding: '1.5rem', color: '#dc2626' }}>{error}</p>;
  if (!data) return null;

  const { item, content } = data;
  const isDecided = item.status === 'decided';

  return (
    <div style={{ maxWidth: 800, margin: '0 auto', padding: '1.5rem' }}>
      {/* Header */}
      <div style={{ marginBottom: '1.5rem' }}>
        <button onClick={() => navigate('/moderation')} style={{ fontSize: '0.8125rem', color: '#6b7280', background: 'none', border: 'none', cursor: 'pointer', marginBottom: 8 }}>
          ← Moderation Queue
        </button>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Content Review</h1>
      </div>

      {/* Content preview */}
      <div style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1.25rem', marginBottom: '1.5rem' }}>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', marginBottom: '0.5rem' }}>
          <span style={{ fontSize: '0.75rem', background: '#dbeafe', color: '#1e40af', borderRadius: '9999px', padding: '0.125rem 0.5rem', fontWeight: 600 }}>
            {content.content_type}
          </span>
          <span style={{ fontSize: '0.75rem', color: '#6b7280' }}>{content.status}</span>
        </div>
        <h2 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.75rem' }}>{content.title}</h2>
        {content.body && (
          <p style={{ fontSize: '0.875rem', color: '#374151', whiteSpace: 'pre-wrap' }}>{content.body}</p>
        )}
        {content.file_name && (
          <p style={{ fontSize: '0.875rem', color: '#6b7280' }}>File: {content.file_name}</p>
        )}
        <p style={{ fontSize: '0.75rem', color: '#9ca3af', marginTop: '0.5rem' }}>
          Submitted by {content.submitted_by.slice(0, 8)}… on {new Date(content.submitted_at).toLocaleString()}
        </p>
      </div>

      {/* Item status */}
      <div style={{ marginBottom: '1.5rem', display: 'flex', gap: '1rem', alignItems: 'center' }}>
        <span style={{ fontSize: '0.875rem' }}>
          Queue status: <strong>{item.status}</strong>
        </span>
        {item.assigned_to && (
          <span style={{ fontSize: '0.875rem', color: '#6b7280' }}>Assigned: {item.assigned_to.slice(0, 8)}…</span>
        )}
        {!item.assigned_to && !isDecided && (
          <button
            onClick={handleAssign}
            style={{ padding: '0.25rem 0.75rem', background: '#4f46e5', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.8125rem', cursor: 'pointer' }}
          >
            Assign to me
          </button>
        )}
      </div>

      {/* Decision form */}
      {!isDecided && (
        <div style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1.25rem' }}>
          <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.75rem' }}>Make a decision</h2>

          <div style={{ display: 'flex', gap: '1rem', marginBottom: '0.75rem' }}>
            {(['approved', 'rejected'] as const).map((d) => (
              <label key={d} style={{ display: 'flex', gap: '0.375rem', alignItems: 'center', cursor: 'pointer', fontSize: '0.875rem' }}>
                <input type="radio" name="decision" value={d} checked={decision === d} onChange={() => setDecision(d)} />
                <span style={{ fontWeight: 600, color: d === 'approved' ? '#065f46' : '#991b1b' }}>{d}</span>
              </label>
            ))}
          </div>

          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>
              Reason {decision === 'rejected' ? '(required)' : '(optional)'}
            </label>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              style={{ width: '100%', padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', minHeight: 80, resize: 'vertical', boxSizing: 'border-box' }}
              placeholder="Explain the decision…"
            />
          </div>

          {decideError && <p style={{ color: '#dc2626', fontSize: '0.875rem', marginBottom: '0.75rem' }}>{decideError}</p>}

          <button
            onClick={handleDecide}
            disabled={deciding || (decision === 'rejected' && !reason.trim())}
            style={{
              padding: '0.5rem 1.25rem',
              background: deciding ? '#a5b4fc' : decision === 'approved' ? '#059669' : '#dc2626',
              color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem',
              cursor: deciding ? 'not-allowed' : 'pointer', fontWeight: 600,
            }}
          >
            {deciding ? 'Saving…' : `Submit ${decision}`}
          </button>
        </div>
      )}

      {/* Decided state */}
      {isDecided && (
        <div style={{ background: item.decision === 'approved' ? '#f0fdf4' : '#fef2f2', border: `1px solid ${item.decision === 'approved' ? '#bbf7d0' : '#fecaca'}`, borderRadius: 8, padding: '1.25rem' }}>
          <p style={{ margin: 0, fontWeight: 600, color: item.decision === 'approved' ? '#065f46' : '#991b1b' }}>
            Decision: {item.decision}
          </p>
          {item.decision_reason && <p style={{ margin: '0.25rem 0 0', fontSize: '0.875rem', color: '#374151' }}>{item.decision_reason}</p>}
          {item.decided_at && <p style={{ margin: '0.25rem 0 0', fontSize: '0.75rem', color: '#6b7280' }}>Decided at {new Date(item.decided_at).toLocaleString()}</p>}
        </div>
      )}
    </div>
  );
}
