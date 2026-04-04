// ProgramDetailPage — program info, enrollment action, roster, and add/drop history.
//
// Immediate validation feedback is shown inline when enrollment is denied:
//   - closed_window, not_published, reader_ineligible
//   - blacklisted, not_whitelisted, prerequisite_not_met
//   - conflict (duplicate / capacity)
//
// The enroll action requires enrollments:write.
// The drop action on individual enrollments requires enrollments:write.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { programsApi, ProgramDetail, Enrollment, EnrollmentHistory } from '../../api/programs';
import { HttpError } from '../../api/client';
import type { PageResult } from '../../api/readers';

const STATUS_STYLE: Record<string, { bg: string; color: string }> = {
  confirmed:  { bg: '#d1fae5', color: '#065f46' },
  cancelled:  { bg: '#fee2e2', color: '#991b1b' },
  waitlisted: { bg: '#fef3c7', color: '#92400e' },
  pending:    { bg: '#f3f4f6', color: '#374151' },
  completed:  { bg: '#dbeafe', color: '#1e40af' },
  no_show:    { bg: '#fce7f3', color: '#9d174d' },
};

function StatusBadge({ status }: { status: string }) {
  const s = STATUS_STYLE[status] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{ display: 'inline-block', padding: '0.125rem 0.5rem', borderRadius: '9999px', fontSize: '0.6875rem', fontWeight: 600, background: s.bg, color: s.color }}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

// Machine-readable denial codes → human labels.
const DENIAL_LABEL: Record<string, string> = {
  closed_window:        'Enrollment window is closed',
  not_published:        'Program is not open for enrollment',
  reader_ineligible:    'Reader account status does not allow enrollment',
  blacklisted:          'Reader is excluded by a blacklist rule',
  not_whitelisted:      'Reader does not meet whitelist criteria',
  prerequisite_not_met: 'Prerequisite program not completed',
  conflict:             'Enrollment conflict',
};

export default function ProgramDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('programs:write');
  const canEnroll = hasPermission('enrollments:write');

  const [prog, setProg] = useState<ProgramDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [seats, setSeats] = useState<number | null>(null);
  const [enrollments, setEnrollments] = useState<PageResult<Enrollment> | null>(null);
  const [loadingEnroll, setLoadingEnroll] = useState(false);
  const [enrollPage, setEnrollPage] = useState(1);

  // Enroll form
  const [readerIDInput, setReaderIDInput] = useState('');
  const [enrollError, setEnrollError] = useState<{ code: string; detail: string } | null>(null);
  const [enrollSuccess, setEnrollSuccess] = useState<Enrollment | null>(null);

  // Drop
  const [droppingId, setDroppingId] = useState<string | null>(null);
  const [dropReason, setDropReason] = useState('');
  const [dropError, setDropError] = useState<string | null>(null);

  // History
  const [historyFor, setHistoryFor] = useState<string | null>(null);
  const [history, setHistory] = useState<EnrollmentHistory[]>([]);

  const fetchProgram = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const [p, s] = await Promise.all([
        programsApi.get(id),
        programsApi.getSeats(id),
      ]);
      setProg(p);
      setSeats(s.remaining_seats);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load program');
    } finally {
      setLoading(false);
    }
  }, [id]);

  const fetchEnrollments = useCallback(async (p: number) => {
    if (!id) return;
    setLoadingEnroll(true);
    try {
      const data = await programsApi.listEnrollmentsByProgram(id, { page: p, per_page: 20 });
      setEnrollments(data);
      setEnrollPage(p);
    } catch { /* ignore */ } finally {
      setLoadingEnroll(false);
    }
  }, [id]);

  useEffect(() => {
    fetchProgram();
    fetchEnrollments(1);
  }, [fetchProgram, fetchEnrollments]);

  const handleEnroll = useCallback(async () => {
    if (!id || !readerIDInput.trim()) return;
    setEnrollError(null);
    setEnrollSuccess(null);
    try {
      const e = await programsApi.enroll(id, readerIDInput.trim());
      setEnrollSuccess(e);
      setReaderIDInput('');
      // Refresh seats and roster.
      const s = await programsApi.getSeats(id);
      setSeats(s.remaining_seats);
      fetchEnrollments(enrollPage);
    } catch (err: unknown) {
      if (err instanceof HttpError) {
        const code = (err.body as { error?: string }).error ?? 'error';
        const detail = (err.body as { detail?: string }).detail ?? err.message;
        setEnrollError({ code, detail });
      } else {
        setEnrollError({ code: 'error', detail: err instanceof Error ? err.message : 'Enrollment failed' });
      }
    }
  }, [id, readerIDInput, enrollPage, fetchEnrollments]);

  const handleDrop = useCallback(async (enrollmentId: string, readerId: string) => {
    setDropError(null);
    try {
      await programsApi.dropEnrollment(enrollmentId, readerId, dropReason);
      setDroppingId(null);
      setDropReason('');
      fetchEnrollments(enrollPage);
      const s = await programsApi.getSeats(id!);
      setSeats(s.remaining_seats);
    } catch (err: unknown) {
      setDropError(err instanceof Error ? err.message : 'Drop failed');
    }
  }, [id, dropReason, enrollPage, fetchEnrollments]);

  const loadHistory = useCallback(async (enrollmentId: string) => {
    if (historyFor === enrollmentId) {
      setHistoryFor(null);
      return;
    }
    const hist = await programsApi.getEnrollmentHistory(enrollmentId);
    setHistory(hist);
    setHistoryFor(enrollmentId);
  }, [historyFor]);

  if (loading) return <p style={{ padding: '1.5rem', color: '#6b7280' }}>Loading…</p>;
  if (error) return <p style={{ padding: '1.5rem', color: '#dc2626' }}>{error}</p>;
  if (!prog) return null;

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', padding: '1.5rem' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <div>
          <button onClick={() => navigate('/programs')} style={{ fontSize: '0.8125rem', color: '#6b7280', background: 'none', border: 'none', cursor: 'pointer', marginBottom: 8 }}>
            ← Programs
          </button>
          <h1 style={{ fontSize: '1.375rem', fontWeight: 700, margin: 0 }}>{prog.title}</h1>
          <span style={{ display: 'inline-block', marginTop: 6, padding: '0.125rem 0.5rem', borderRadius: '9999px', fontSize: '0.75rem', fontWeight: 600, background: prog.status === 'published' ? '#d1fae5' : '#f3f4f6', color: prog.status === 'published' ? '#065f46' : '#374151' }}>
            {prog.status}
          </span>
        </div>
        {canWrite && (
          <button
            onClick={() => navigate(`/programs/${prog.id}/edit`)}
            style={{ padding: '0.5rem 1rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}
          >
            Edit
          </button>
        )}
      </div>

      {/* Info grid */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '0.75rem', marginBottom: '1.5rem' }}>
        <InfoCard label="Starts" value={new Date(prog.starts_at).toLocaleString()} />
        <InfoCard label="Ends" value={new Date(prog.ends_at).toLocaleString()} />
        <InfoCard label="Capacity" value={String(prog.capacity)} />
        <InfoCard
          label="Remaining seats"
          value={seats != null ? String(seats) : '—'}
          valueColor={seats != null && seats === 0 ? '#dc2626' : undefined}
        />
        {prog.enrollment_opens_at && <InfoCard label="Enrollment opens" value={new Date(prog.enrollment_opens_at).toLocaleString()} />}
        {prog.enrollment_closes_at && <InfoCard label="Enrollment closes" value={new Date(prog.enrollment_closes_at).toLocaleString()} />}
      </div>

      {prog.description && (
        <p style={{ color: '#374151', fontSize: '0.875rem', marginBottom: '1.5rem' }}>{prog.description}</p>
      )}

      {/* Prerequisites */}
      {prog.prerequisites.length > 0 && (
        <section style={{ marginBottom: '1.5rem' }}>
          <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.5rem' }}>Prerequisites</h2>
          <ul style={{ margin: 0, paddingLeft: '1.25rem', fontSize: '0.875rem', color: '#374151' }}>
            {prog.prerequisites.map((pr) => (
              <li key={pr.id}>{pr.description ?? pr.required_program_id}</li>
            ))}
          </ul>
        </section>
      )}

      {/* Enrollment rules */}
      {prog.enrollment_rules.length > 0 && (
        <section style={{ marginBottom: '1.5rem' }}>
          <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.5rem' }}>Enrollment rules</h2>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.375rem' }}>
            {prog.enrollment_rules.map((rule) => (
              <div key={rule.id} style={{ fontSize: '0.8125rem', display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                <span style={{
                  padding: '0.125rem 0.375rem', borderRadius: '9999px', fontSize: '0.6875rem', fontWeight: 600,
                  background: rule.rule_type === 'whitelist' ? '#d1fae5' : '#fee2e2',
                  color: rule.rule_type === 'whitelist' ? '#065f46' : '#991b1b',
                }}>
                  {rule.rule_type}
                </span>
                <span style={{ fontFamily: 'monospace' }}>{rule.match_field} = {rule.match_value}</span>
                {rule.reason && <span style={{ color: '#6b7280' }}>— {rule.reason}</span>}
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Enroll reader */}
      {canEnroll && prog.status === 'published' && (
        <section style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1.25rem', marginBottom: '1.5rem' }}>
          <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.75rem' }}>Enroll a reader</h2>
          <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center', flexWrap: 'wrap' }}>
            <input
              value={readerIDInput}
              onChange={(e) => setReaderIDInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleEnroll()}
              placeholder="Reader ID (UUID)"
              style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', width: 280, fontFamily: 'monospace' }}
            />
            <button
              onClick={handleEnroll}
              disabled={!readerIDInput.trim()}
              style={{ padding: '0.375rem 0.875rem', background: '#059669', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: readerIDInput.trim() ? 'pointer' : 'not-allowed', opacity: readerIDInput.trim() ? 1 : 0.6 }}
            >
              Enroll
            </button>
          </div>

          {enrollError && (
            <div style={{ marginTop: '0.75rem', background: '#fef2f2', border: '1px solid #fecaca', borderRadius: 6, padding: '0.75rem' }}>
              <p style={{ margin: 0, fontWeight: 600, color: '#991b1b', fontSize: '0.8125rem' }}>
                {DENIAL_LABEL[enrollError.code] ?? 'Enrollment denied'}
              </p>
              <p style={{ margin: '0.25rem 0 0', color: '#7f1d1d', fontSize: '0.8125rem' }}>{enrollError.detail}</p>
            </div>
          )}

          {enrollSuccess && (
            <div style={{ marginTop: '0.75rem', background: '#f0fdf4', border: '1px solid #bbf7d0', borderRadius: 6, padding: '0.75rem' }}>
              <p style={{ margin: 0, color: '#166534', fontWeight: 600, fontSize: '0.8125rem' }}>
                Enrolled successfully — {enrollSuccess.remaining_seats ?? '?'} seat(s) remaining.
              </p>
            </div>
          )}
        </section>
      )}

      {/* Enrollment roster */}
      <section>
        <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginBottom: '0.75rem' }}>
          Enrollment roster {enrollments ? `(${enrollments.total})` : ''}
        </h2>

        {loadingEnroll && <p style={{ color: '#6b7280', fontSize: '0.875rem' }}>Loading…</p>}

        {enrollments && enrollments.items.length === 0 && !loadingEnroll && (
          <p style={{ color: '#6b7280', fontSize: '0.875rem' }}>No enrollments yet.</p>
        )}

        {enrollments && enrollments.items.length > 0 && (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr style={{ background: '#f3f4f6', borderBottom: '1px solid #e5e7eb' }}>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Reader ID</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Status</th>
                  <th style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>Enrolled at</th>
                  <th style={{ padding: '0.5rem 0.75rem' }}></th>
                </tr>
              </thead>
              <tbody>
                {enrollments.items.map((e) => (
                  <>
                    <tr key={e.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                      <td style={{ padding: '0.5rem 0.75rem', fontFamily: 'monospace', fontSize: '0.75rem' }}>{e.reader_id}</td>
                      <td style={{ padding: '0.5rem 0.75rem' }}><StatusBadge status={e.status} /></td>
                      <td style={{ padding: '0.5rem 0.75rem', color: '#6b7280', fontSize: '0.75rem' }}>
                        {new Date(e.enrolled_at).toLocaleString()}
                      </td>
                      <td style={{ padding: '0.5rem 0.75rem', display: 'flex', gap: '0.5rem' }}>
                        <button
                          onClick={() => loadHistory(e.id)}
                          style={{ fontSize: '0.75rem', color: '#4f46e5', background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline' }}
                        >
                          {historyFor === e.id ? 'Hide history' : 'History'}
                        </button>
                        {canEnroll && e.status === 'confirmed' && (
                          droppingId === e.id ? (
                            <span style={{ display: 'flex', gap: '0.375rem', alignItems: 'center' }}>
                              <input
                                value={dropReason}
                                onChange={(ev) => setDropReason(ev.target.value)}
                                placeholder="Reason (optional)"
                                style={{ padding: '0.25rem 0.375rem', border: '1px solid #d1d5db', borderRadius: 4, fontSize: '0.75rem', width: 140 }}
                              />
                              <button onClick={() => handleDrop(e.id, e.reader_id)} style={{ fontSize: '0.75rem', color: '#dc2626', background: 'none', border: 'none', cursor: 'pointer', fontWeight: 600 }}>Confirm drop</button>
                              <button onClick={() => setDroppingId(null)} style={{ fontSize: '0.75rem', color: '#6b7280', background: 'none', border: 'none', cursor: 'pointer' }}>Cancel</button>
                            </span>
                          ) : (
                            <button
                              onClick={() => { setDroppingId(e.id); setDropReason(''); setDropError(null); }}
                              style={{ fontSize: '0.75rem', color: '#dc2626', background: 'none', border: 'none', cursor: 'pointer', textDecoration: 'underline' }}
                            >
                              Drop
                            </button>
                          )
                        )}
                      </td>
                    </tr>

                    {/* Inline history panel */}
                    {historyFor === e.id && history.length > 0 && (
                      <tr key={`hist-${e.id}`}>
                        <td colSpan={4} style={{ padding: '0 0.75rem 0.75rem', background: '#fafafa' }}>
                          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.75rem' }}>
                            <thead>
                              <tr style={{ color: '#9ca3af' }}>
                                <th style={{ textAlign: 'left', padding: '0.25rem 0.5rem' }}>From</th>
                                <th style={{ textAlign: 'left', padding: '0.25rem 0.5rem' }}>To</th>
                                <th style={{ textAlign: 'left', padding: '0.25rem 0.5rem' }}>Reason</th>
                                <th style={{ textAlign: 'left', padding: '0.25rem 0.5rem' }}>At</th>
                              </tr>
                            </thead>
                            <tbody>
                              {history.map((h) => (
                                <tr key={h.id}>
                                  <td style={{ padding: '0.25rem 0.5rem' }}>{h.previous_status}</td>
                                  <td style={{ padding: '0.25rem 0.5rem' }}>{h.new_status}</td>
                                  <td style={{ padding: '0.25rem 0.5rem', color: '#6b7280' }}>{h.reason ?? '—'}</td>
                                  <td style={{ padding: '0.25rem 0.5rem', color: '#6b7280' }}>{new Date(h.changed_at).toLocaleString()}</td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </td>
                      </tr>
                    )}
                  </>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {dropError && <p style={{ color: '#dc2626', fontSize: '0.8125rem', marginTop: 8 }}>{dropError}</p>}

        {enrollments && enrollments.total_pages > 1 && (
          <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.75rem', alignItems: 'center' }}>
            <button onClick={() => fetchEnrollments(enrollPage - 1)} disabled={enrollPage <= 1} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>← Prev</button>
            <span style={{ fontSize: '0.8125rem' }}>Page {enrollPage} of {enrollments.total_pages}</span>
            <button onClick={() => fetchEnrollments(enrollPage + 1)} disabled={enrollPage >= enrollments.total_pages} style={{ padding: '0.25rem 0.5rem', fontSize: '0.8125rem' }}>Next →</button>
          </div>
        )}
      </section>
    </div>
  );
}

function InfoCard({ label, value, valueColor }: { label: string; value: string; valueColor?: string }) {
  return (
    <div style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '0.75rem 1rem' }}>
      <p style={{ margin: 0, fontSize: '0.75rem', color: '#6b7280' }}>{label}</p>
      <p style={{ margin: 0, fontSize: '1rem', fontWeight: 600, color: valueColor ?? '#111827' }}>{value}</p>
    </div>
  );
}
