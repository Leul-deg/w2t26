// ReaderDetailPage — profile, sensitive fields, current holdings, and loan history
// for a single reader.
//
// Security notes:
//  - Sensitive fields are masked by default; revealed only via StepUpModal flow.
//  - The reveal button is hidden for users without readers:reveal_sensitive.
//  - Status changes require readers:write permission.
//  - All reveals are audit-logged server-side.

import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { readersApi, Reader, LoanHistoryItem, PageResult } from '../../api/readers';
import MaskedField from '../../components/MaskedField';
import LoadingState from '../../components/LoadingState';
import ErrorState from '../../components/ErrorState';
import { HttpError } from '../../api/client';

// ── Status badge ──────────────────────────────────────────────────────────────

const STATUS_COLOUR: Record<string, { bg: string; color: string }> = {
  active:               { bg: '#d1fae5', color: '#065f46' },
  frozen:               { bg: '#dbeafe', color: '#1e40af' },
  blacklisted:          { bg: '#fee2e2', color: '#991b1b' },
  pending_verification: { bg: '#fef3c7', color: '#92400e' },
};

function StatusBadge({ code }: { code: string }) {
  const style = STATUS_COLOUR[code] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{
      display: 'inline-block',
      padding: '0.1875rem 0.625rem',
      borderRadius: '9999px',
      fontSize: '0.75rem',
      fontWeight: 600,
      background: style.bg,
      color: style.color,
    }}>
      {code.replace('_', ' ')}
    </span>
  );
}

// ── Section card ─────────────────────────────────────────────────────────────

function SectionCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{
      background: '#fff',
      border: '1px solid #e5e7eb',
      borderRadius: '6px',
      overflow: 'hidden',
      marginBottom: '1rem',
    }}>
      <div style={{
        padding: '0.625rem 1rem',
        borderBottom: '1px solid #f3f4f6',
        fontWeight: 600,
        fontSize: '0.8125rem',
        color: '#374151',
        background: '#f9fafb',
      }}>
        {title}
      </div>
      <div style={{ padding: '0.875rem 1rem' }}>
        {children}
      </div>
    </div>
  );
}

// ── Field row ─────────────────────────────────────────────────────────────────

function FieldRow({ label, value }: { label: string; value?: string | null }) {
  return (
    <div style={{ display: 'flex', gap: '1rem', padding: '0.25rem 0', fontSize: '0.8125rem' }}>
      <span style={{ minWidth: '140px', color: '#6b7280', fontWeight: 500, textTransform: 'uppercase', fontSize: '0.6875rem', letterSpacing: '0.04em', paddingTop: '0.125rem' }}>
        {label}
      </span>
      <span style={{ color: value ? '#1a1a2e' : '#9ca3af', fontStyle: value ? undefined : 'italic' }}>
        {value ?? '—'}
      </span>
    </div>
  );
}

// ── Loan history tab ──────────────────────────────────────────────────────────

const EVENT_LABELS: Record<string, string> = {
  checkout:        'Checkout',
  return:          'Returned',
  renewal:         'Renewed',
  hold_placed:     'Hold placed',
  hold_cancelled:  'Hold cancelled',
  transit_out:     'Transit out',
  transit_in:      'Transit in',
};

function LoanHistoryTab({ readerId }: { readerId: string }) {
  const [result, setResult] = useState<PageResult<LoanHistoryItem> | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    readersApi.getLoanHistory(readerId, page)
      .then(setResult)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load history'))
      .finally(() => setLoading(false));
  }, [readerId, page]);

  if (loading) return <LoadingState />;
  if (error) return <ErrorState message={error} />;
  if (!result || result.total === 0) {
    return <div style={{ color: '#6b7280', fontSize: '0.8125rem', padding: '0.5rem 0' }}>No loan history.</div>;
  }

  return (
    <div>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
        <thead>
          <tr style={{ background: '#f9fafb' }}>
            {['Date', 'Event', 'Title', 'Barcode', 'Due date', 'Returned'].map((h) => (
              <th key={h} style={{ padding: '0.375rem 0.75rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', color: '#6b7280', borderBottom: '2px solid #e5e7eb' }}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {result.items.map((item) => (
            <tr key={item.event_id} style={{ borderBottom: '1px solid #f3f4f6' }}>
              <td style={{ padding: '0.375rem 0.75rem', whiteSpace: 'nowrap' }}>
                {new Date(item.created_at).toLocaleDateString()}
              </td>
              <td style={{ padding: '0.375rem 0.75rem' }}>
                {EVENT_LABELS[item.event_type] ?? item.event_type}
              </td>
              <td style={{ padding: '0.375rem 0.75rem' }}>{item.title}</td>
              <td style={{ padding: '0.375rem 0.75rem', fontFamily: 'monospace' }}>{item.barcode}</td>
              <td style={{ padding: '0.375rem 0.75rem' }}>{item.due_date ?? '—'}</td>
              <td style={{ padding: '0.375rem 0.75rem' }}>
                {item.returned_at ? new Date(item.returned_at).toLocaleDateString() : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {result.total_pages > 1 && (
        <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end', paddingTop: '0.625rem', fontSize: '0.8125rem' }}>
          <button disabled={page <= 1} onClick={() => setPage(page - 1)} style={{ padding: '0.25rem 0.5rem', border: '1px solid #d1d5db', borderRadius: '4px', cursor: page <= 1 ? 'default' : 'pointer', opacity: page <= 1 ? 0.4 : 1, background: '#fff' }}>‹</button>
          <span style={{ padding: '0.25rem 0.5rem', color: '#6b7280' }}>{page} / {result.total_pages}</span>
          <button disabled={page >= result.total_pages} onClick={() => setPage(page + 1)} style={{ padding: '0.25rem 0.5rem', border: '1px solid #d1d5db', borderRadius: '4px', cursor: page >= result.total_pages ? 'default' : 'pointer', opacity: page >= result.total_pages ? 0.4 : 1, background: '#fff' }}>›</button>
        </div>
      )}
    </div>
  );
}

// ── Current holdings tab ──────────────────────────────────────────────────────

function CurrentHoldingsTab({ readerId }: { readerId: string }) {
  const [items, setItems] = useState<LoanHistoryItem[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    readersApi.getCurrentHoldings(readerId)
      .then(setItems)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load holdings'))
      .finally(() => setLoading(false));
  }, [readerId]);

  if (loading) return <LoadingState />;
  if (error) return <ErrorState message={error} />;
  if (!items || items.length === 0) {
    return <div style={{ color: '#6b7280', fontSize: '0.8125rem', padding: '0.5rem 0' }}>No items currently on loan.</div>;
  }

  const today = new Date();

  return (
    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
      <thead>
        <tr style={{ background: '#f9fafb' }}>
          {['Title', 'Barcode', 'Due date', 'Overdue'].map((h) => (
            <th key={h} style={{ padding: '0.375rem 0.75rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', color: '#6b7280', borderBottom: '2px solid #e5e7eb' }}>
              {h}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {items.map((item) => {
          const isOverdue = item.due_date ? new Date(item.due_date) < today : false;
          return (
            <tr key={item.event_id} style={{ borderBottom: '1px solid #f3f4f6' }}>
              <td style={{ padding: '0.375rem 0.75rem' }}>{item.title}</td>
              <td style={{ padding: '0.375rem 0.75rem', fontFamily: 'monospace' }}>{item.barcode}</td>
              <td style={{ padding: '0.375rem 0.75rem', color: isOverdue ? '#dc2626' : undefined }}>
                {item.due_date ?? '—'}
              </td>
              <td style={{ padding: '0.375rem 0.75rem' }}>
                {isOverdue ? (
                  <span style={{ color: '#dc2626', fontWeight: 600, fontSize: '0.6875rem' }}>OVERDUE</span>
                ) : '—'}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

// ── Status change modal ───────────────────────────────────────────────────────

const ALL_STATUSES = [
  { code: 'active',               label: 'Active' },
  { code: 'frozen',               label: 'Frozen' },
  { code: 'blacklisted',          label: 'Blacklisted' },
  { code: 'pending_verification', label: 'Pending Verification' },
];

function StatusChangeModal({
  currentStatus,
  onConfirm,
  onCancel,
  submitting,
}: {
  currentStatus: string;
  onConfirm: (code: string) => void;
  onCancel: () => void;
  submitting: boolean;
}) {
  const [selected, setSelected] = useState(currentStatus);

  return (
    <div role="dialog" aria-modal="true" style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 10000 }}
      onClick={(e) => { if (e.target === e.currentTarget) onCancel(); }}>
      <div style={{ background: '#fff', borderRadius: '6px', padding: '1.5rem', width: '100%', maxWidth: '360px', boxShadow: '0 8px 32px rgba(0,0,0,0.18)' }}>
        <div style={{ fontWeight: 700, fontSize: '1rem', marginBottom: '1rem', color: '#1a1a2e' }}>Change Reader Status</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', marginBottom: '1.25rem' }}>
          {ALL_STATUSES.map((s) => (
            <label key={s.code} style={{ display: 'flex', alignItems: 'center', gap: '0.625rem', cursor: 'pointer', fontSize: '0.875rem' }}>
              <input type="radio" name="status" value={s.code} checked={selected === s.code} onChange={() => setSelected(s.code)} />
              {s.label}
            </label>
          ))}
        </div>
        <div style={{ display: 'flex', gap: '0.625rem', justifyContent: 'flex-end' }}>
          <button onClick={onCancel} style={{ padding: '0.5rem 1rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: '4px', cursor: 'pointer', fontSize: '0.875rem' }}>
            Cancel
          </button>
          <button
            disabled={submitting || selected === currentStatus}
            onClick={() => onConfirm(selected)}
            style={{ padding: '0.5rem 1rem', background: '#2563eb', color: '#fff', border: 'none', borderRadius: '4px', cursor: submitting ? 'not-allowed' : 'pointer', fontSize: '0.875rem', fontWeight: 600, opacity: submitting || selected === currentStatus ? 0.6 : 1 }}
          >
            {submitting ? 'Saving…' : 'Confirm'}
          </button>
        </div>
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

type Tab = 'profile' | 'holdings' | 'history';

export default function ReaderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { hasPermission } = useAuth();

  const canReveal = hasPermission('readers:reveal_sensitive');
  const canWrite  = hasPermission('readers:write');

  const [reader, setReader] = useState<Reader | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [activeTab, setActiveTab] = useState<Tab>('profile');
  const [showStatusModal, setShowStatusModal] = useState(false);
  const [statusSubmitting, setStatusSubmitting] = useState(false);
  const [statusError, setStatusError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    readersApi.get(id)
      .then(setReader)
      .catch((err) => {
        if (err instanceof HttpError && err.status === 404) {
          setError('Reader not found.');
        } else {
          setError(err instanceof Error ? err.message : 'Failed to load reader');
        }
      })
      .finally(() => setLoading(false));
  }, [id]);

  async function handleStatusChange(code: string) {
    if (!id) return;
    setStatusSubmitting(true);
    setStatusError(null);
    try {
      await readersApi.updateStatus(id, code);
      setReader((r) => r ? { ...r, status_code: code } : r);
      setShowStatusModal(false);
    } catch (err) {
      setStatusError(err instanceof Error ? err.message : 'Failed to update status');
    } finally {
      setStatusSubmitting(false);
    }
  }

  if (loading) return <LoadingState />;
  if (error || !reader) return <ErrorState message={error ?? 'Reader not found'} onRetry={() => navigate('/readers')} />;

  const fullName = reader.preferred_name
    ? `${reader.first_name} "${reader.preferred_name}" ${reader.last_name}`
    : `${reader.first_name} ${reader.last_name}`;

  const TAB_STYLE = (t: Tab): React.CSSProperties => ({
    padding: '0.5rem 1rem',
    fontSize: '0.875rem',
    fontWeight: activeTab === t ? 600 : 400,
    color: activeTab === t ? '#2563eb' : '#374151',
    background: 'none',
    border: 'none',
    borderBottom: activeTab === t ? '2px solid #2563eb' : '2px solid transparent',
    cursor: 'pointer',
  });

  return (
    <div>
      {/* Back link */}
      <button
        onClick={() => navigate('/readers')}
        style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', fontSize: '0.8125rem', padding: '0 0 0.75rem 0' }}
      >
        ← Back to readers
      </button>

      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '1rem', flexWrap: 'wrap', gap: '0.5rem' }}>
        <div>
          <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#1a1a2e' }}>{fullName}</div>
          <div style={{ fontSize: '0.8125rem', color: '#6b7280', marginTop: '0.25rem', display: 'flex', alignItems: 'center', gap: '0.625rem' }}>
            <span style={{ fontFamily: 'monospace' }}>{reader.reader_number}</span>
            <StatusBadge code={reader.status_code} />
          </div>
        </div>

        {canWrite && (
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button
              onClick={() => navigate(`/readers/${reader.id}/edit`)}
              style={{ padding: '0.4375rem 0.875rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: '4px', cursor: 'pointer', fontSize: '0.8125rem' }}
            >
              Edit
            </button>
            <button
              onClick={() => setShowStatusModal(true)}
              style={{ padding: '0.4375rem 0.875rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: '4px', cursor: 'pointer', fontSize: '0.8125rem' }}
            >
              Change status
            </button>
          </div>
        )}
      </div>

      {statusError && (
        <div role="alert" style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.625rem 0.875rem', fontSize: '0.8125rem', color: '#dc2626', marginBottom: '0.75rem' }}>
          {statusError}
        </div>
      )}

      {/* Tab navigation */}
      <div style={{ display: 'flex', borderBottom: '1px solid #e5e7eb', marginBottom: '1rem' }}>
        <button style={TAB_STYLE('profile')} onClick={() => setActiveTab('profile')}>Profile</button>
        <button style={TAB_STYLE('holdings')} onClick={() => setActiveTab('holdings')}>Current Holdings</button>
        <button style={TAB_STYLE('history')} onClick={() => setActiveTab('history')}>Loan History</button>
      </div>

      {/* Profile tab */}
      {activeTab === 'profile' && (
        <>
          <SectionCard title="Basic Information">
            <FieldRow label="First name"  value={reader.first_name} />
            <FieldRow label="Last name"   value={reader.last_name} />
            <FieldRow label="Preferred name" value={reader.preferred_name} />
            <FieldRow label="Registered"  value={new Date(reader.registered_at).toLocaleDateString()} />
            <FieldRow label="Branch"      value={reader.branch_id} />
            <FieldRow label="Notes"       value={reader.notes} />
          </SectionCard>

          <SectionCard title="Sensitive Information">
            <div style={{ fontSize: '0.75rem', color: '#6b7280', marginBottom: '0.75rem' }}>
              Sensitive fields are encrypted at rest. Values are masked by default and require
              step-up authentication to reveal. All reveals are audit-logged.
            </div>

            <MaskedField
              label="National ID"
              fieldKey="national_id"
              resourceId={reader.id}
              revealEndpoint="/readers"
              canReveal={canReveal && reader.sensitive_fields?.national_id !== undefined}
            />
            <MaskedField
              label="Contact Email"
              fieldKey="contact_email"
              resourceId={reader.id}
              revealEndpoint="/readers"
              canReveal={canReveal && reader.sensitive_fields?.contact_email !== undefined}
            />
            <MaskedField
              label="Contact Phone"
              fieldKey="contact_phone"
              resourceId={reader.id}
              revealEndpoint="/readers"
              canReveal={canReveal && reader.sensitive_fields?.contact_phone !== undefined}
            />
            <MaskedField
              label="Date of Birth"
              fieldKey="date_of_birth"
              resourceId={reader.id}
              revealEndpoint="/readers"
              canReveal={canReveal && reader.sensitive_fields?.date_of_birth !== undefined}
            />
          </SectionCard>
        </>
      )}

      {/* Current holdings tab */}
      {activeTab === 'holdings' && (
        <SectionCard title="Currently On Loan">
          <CurrentHoldingsTab readerId={reader.id} />
        </SectionCard>
      )}

      {/* History tab */}
      {activeTab === 'history' && (
        <SectionCard title="Loan History">
          <LoanHistoryTab readerId={reader.id} />
        </SectionCard>
      )}

      {/* Status change modal */}
      {showStatusModal && (
        <StatusChangeModal
          currentStatus={reader.status_code}
          onConfirm={handleStatusChange}
          onCancel={() => { setShowStatusModal(false); setStatusError(null); }}
          submitting={statusSubmitting}
        />
      )}
    </div>
  );
}
