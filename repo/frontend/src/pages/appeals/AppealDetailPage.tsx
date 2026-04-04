// AppealDetailPage — view an appeal and its arbitration record.
// Moderators (appeals:read) can mark under review; arbitrators (appeals:decide) can arbitrate.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { appealsApi, AppealDetail } from '../../api/appeals';

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  submitted:    { bg: '#fef3c7', color: '#92400e' },
  under_review: { bg: '#dbeafe', color: '#1e40af' },
  resolved:     { bg: '#d1fae5', color: '#065f46' },
  dismissed:    { bg: '#fee2e2', color: '#991b1b' },
};

const APPEAL_TYPE_LABELS: Record<string, string> = {
  enrollment_denial: 'Enrollment denial',
  account_suspension: 'Account suspension',
  feedback_rejection: 'Feedback rejection',
  blacklist_removal: 'Blacklist removal',
  other: 'Other',
};

export default function AppealDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canRead = hasPermission('appeals:read');
  const canDecide = hasPermission('appeals:decide');

  const [data, setData] = useState<AppealDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Arbitration form state
  const [arbDecision, setArbDecision] = useState<'upheld' | 'dismissed' | 'partial'>('upheld');
  const [arbNotes, setArbNotes] = useState('');
  const [arbitrating, setArbitrating] = useState(false);
  const [arbError, setArbError] = useState<string | null>(null);

  // Review action state
  const [reviewing, setReviewing] = useState(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const d = await appealsApi.get(id);
      setData(d);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const handleReview = async () => {
    if (!id) return;
    setReviewing(true);
    try {
      await appealsApi.review(id);
      load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Review action failed');
    } finally {
      setReviewing(false);
    }
  };

  const handleArbitrate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    setArbError(null);
    setArbitrating(true);
    try {
      const updated = await appealsApi.arbitrate(id, {
        decision: arbDecision,
        decision_notes: arbNotes.trim(),
      });
      setData(updated);
    } catch (err: unknown) {
      setArbError(err instanceof Error ? err.message : 'Arbitration failed');
    } finally {
      setArbitrating(false);
    }
  };

  if (loading) return <p style={{ padding: '1.5rem', color: '#6b7280' }}>Loading…</p>;
  if (error) return <p style={{ padding: '1.5rem', color: '#dc2626' }}>{error}</p>;
  if (!data) return null;

  const { appeal, arbitration } = data;
  const statusStyle = STATUS_STYLE[appeal.status] ?? { bg: '#f3f4f6', color: '#374151' };
  const isTerminal = appeal.status === 'resolved' || appeal.status === 'dismissed';

  return (
    <div style={{ maxWidth: 800, margin: '0 auto', padding: '1.5rem' }}>
      {/* Header */}
      <div style={{ marginBottom: '1.5rem' }}>
        <button onClick={() => navigate('/appeals')}
          style={{ fontSize: '0.8125rem', color: '#6b7280', background: 'none', border: 'none', cursor: 'pointer', marginBottom: 8 }}>
          ← Appeals
        </button>
        <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
          <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Appeal</h1>
          <span style={{ fontSize: '0.75rem', padding: '0.125rem 0.5rem', borderRadius: '9999px', background: statusStyle.bg, color: statusStyle.color, fontWeight: 600 }}>
            {appeal.status.replace(/_/g, ' ')}
          </span>
        </div>
      </div>

      {/* Appeal details */}
      <div style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1.25rem', marginBottom: '1.5rem' }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem 1.5rem', marginBottom: '0.75rem' }}>
          <div>
            <p style={{ margin: 0, fontSize: '0.75rem', color: '#6b7280', fontWeight: 600 }}>Reader</p>
            <p style={{ margin: '0.125rem 0 0', fontSize: '0.875rem', fontFamily: 'monospace' }}>{appeal.reader_id}</p>
          </div>
          <div>
            <p style={{ margin: 0, fontSize: '0.75rem', color: '#6b7280', fontWeight: 600 }}>Type</p>
            <p style={{ margin: '0.125rem 0 0', fontSize: '0.875rem' }}>{APPEAL_TYPE_LABELS[appeal.appeal_type] ?? appeal.appeal_type}</p>
          </div>
          {appeal.target_type && (
            <div>
              <p style={{ margin: 0, fontSize: '0.75rem', color: '#6b7280', fontWeight: 600 }}>Target</p>
              <p style={{ margin: '0.125rem 0 0', fontSize: '0.875rem', fontFamily: 'monospace' }}>{appeal.target_type} · {appeal.target_id}</p>
            </div>
          )}
          <div>
            <p style={{ margin: 0, fontSize: '0.75rem', color: '#6b7280', fontWeight: 600 }}>Submitted</p>
            <p style={{ margin: '0.125rem 0 0', fontSize: '0.875rem' }}>{new Date(appeal.submitted_at).toLocaleString()}</p>
          </div>
        </div>
        <div>
          <p style={{ margin: 0, fontSize: '0.75rem', color: '#6b7280', fontWeight: 600 }}>Reason</p>
          <p style={{ margin: '0.25rem 0 0', fontSize: '0.875rem', color: '#374151', whiteSpace: 'pre-wrap' }}>{appeal.reason}</p>
        </div>
      </div>

      {/* Mark under review */}
      {canRead && appeal.status === 'submitted' && (
        <div style={{ marginBottom: '1.5rem' }}>
          <button onClick={handleReview} disabled={reviewing}
            style={{ padding: '0.5rem 1rem', background: '#4f46e5', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer', fontWeight: 600 }}>
            {reviewing ? 'Updating…' : 'Mark under review'}
          </button>
        </div>
      )}

      {/* Arbitration record */}
      {arbitration && (
        <div style={{
          background: arbitration.decision === 'upheld' ? '#f0fdf4' : arbitration.decision === 'dismissed' ? '#fef2f2' : '#fefce8',
          border: `1px solid ${arbitration.decision === 'upheld' ? '#bbf7d0' : arbitration.decision === 'dismissed' ? '#fecaca' : '#fef08a'}`,
          borderRadius: 8, padding: '1.25rem', marginBottom: '1.5rem'
        }}>
          <p style={{ margin: 0, fontWeight: 700, fontSize: '0.9375rem', color: arbitration.decision === 'upheld' ? '#065f46' : arbitration.decision === 'dismissed' ? '#991b1b' : '#854d0e' }}>
            Arbitration: {arbitration.decision}
          </p>
          {arbitration.decision_notes && (
            <p style={{ margin: '0.375rem 0 0', fontSize: '0.875rem', color: '#374151', whiteSpace: 'pre-wrap' }}>{arbitration.decision_notes}</p>
          )}
          <p style={{ margin: '0.375rem 0 0', fontSize: '0.75rem', color: '#6b7280' }}>
            Arbitrated by {arbitration.arbitrator_id.slice(0, 8)}… on {new Date(arbitration.decided_at).toLocaleString()}
          </p>
        </div>
      )}

      {/* Arbitrate form — only when in under_review and not yet decided */}
      {canDecide && appeal.status === 'under_review' && !arbitration && (
        <div style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1.25rem' }}>
          <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.75rem' }}>Arbitrate</h2>
          <form onSubmit={handleArbitrate}>
            <div style={{ display: 'flex', gap: '1.25rem', marginBottom: '0.75rem' }}>
              {(['upheld', 'partial', 'dismissed'] as const).map((d) => (
                <label key={d} style={{ display: 'flex', gap: '0.375rem', alignItems: 'center', cursor: 'pointer', fontSize: '0.875rem' }}>
                  <input type="radio" name="arbDecision" value={d} checked={arbDecision === d} onChange={() => setArbDecision(d)} />
                  <span style={{ fontWeight: 600, color: d === 'upheld' ? '#065f46' : d === 'dismissed' ? '#991b1b' : '#854d0e' }}>{d}</span>
                </label>
              ))}
            </div>
            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Decision notes *</label>
              <textarea
                value={arbNotes}
                onChange={(e) => setArbNotes(e.target.value)}
                required
                style={{ width: '100%', padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', minHeight: 80, resize: 'vertical', boxSizing: 'border-box' }}
                placeholder="Provide justification for this decision…"
              />
            </div>
            {arbError && <p style={{ color: '#dc2626', fontSize: '0.875rem', marginBottom: '0.75rem' }}>{arbError}</p>}
            <button
              type="submit"
              disabled={arbitrating || !arbNotes.trim()}
              style={{
                padding: '0.5rem 1.25rem',
                background: arbitrating ? '#a5b4fc' : arbDecision === 'upheld' ? '#059669' : arbDecision === 'dismissed' ? '#dc2626' : '#d97706',
                color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem',
                cursor: arbitrating ? 'not-allowed' : 'pointer', fontWeight: 600,
              }}
            >
              {arbitrating ? 'Saving…' : `Submit arbitration (${arbDecision})`}
            </button>
          </form>
        </div>
      )}

      {/* Terminal state notice */}
      {isTerminal && !arbitration && (
        <p style={{ color: '#6b7280', fontSize: '0.875rem' }}>This appeal has been {appeal.status}.</p>
      )}
    </div>
  );
}
