// CirculationPage — checkout/return workstation and circulation history.

import { useEffect, useRef, useState } from 'react';
import { useAuth } from '../../auth/AuthContext';
import { circulationApi, CirculationEvent, PageResult } from '../../api/circulation';

// ── Helpers ───────────────────────────────────────────────────────────────────

function defaultDueDate(): string {
  const d = new Date();
  d.setDate(d.getDate() + 14);
  return d.toISOString().slice(0, 10);
}

function EventTypeBadge({ type }: { type: string }) {
  const colours: Record<string, { bg: string; color: string }> = {
    checkout: { bg: '#dbeafe', color: '#1e40af' },
    return:   { bg: '#d1fae5', color: '#065f46' },
    renewal:  { bg: '#fef3c7', color: '#92400e' },
  };
  const style = colours[type] ?? { bg: '#f3f4f6', color: '#374151' };
  return (
    <span style={{
      display: 'inline-block',
      padding: '0.125rem 0.5rem',
      borderRadius: '9999px',
      fontSize: '0.6875rem',
      fontWeight: 600,
      background: style.bg,
      color: style.color,
      textTransform: 'capitalize',
    }}>
      {type}
    </span>
  );
}

// ── Checkout tab ──────────────────────────────────────────────────────────────

function CheckoutTab({ isAdmin }: { isAdmin: boolean }) {
  const barcodeRef = useRef<HTMLInputElement>(null);
  const [barcode, setBarcode] = useState('');
  const [readerId, setReaderId] = useState('');
  const [branchId, setBranchId] = useState('');
  const [dueDate, setDueDate] = useState(defaultDueDate);
  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    barcodeRef.current?.focus();
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const b = barcode.trim();
    const r = readerId.trim();
    if (!b || !r) { setError('Barcode and reader ID are required'); return; }
    if (isAdmin && !branchId.trim()) { setError('Branch ID is required for administrator accounts'); return; }
    setSubmitting(true);
    setSuccess(null);
    setError(null);
    try {
      const event = await circulationApi.checkout({
        barcode: b,
        reader_id: r,
        due_date: dueDate,
        ...(isAdmin && branchId.trim() ? { branch_id: branchId.trim() } : {}),
      });
      setSuccess(`Checked out: ${event.copy_id} to reader ${event.reader_id} (due ${event.due_date ?? dueDate})`);
      setBarcode('');
      setReaderId('');
      setDueDate(defaultDueDate());
      barcodeRef.current?.focus();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Checkout failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1rem', maxWidth: '480px' }}>
      {success && (
        <div style={{ background: '#d1fae5', border: '1px solid #6ee7b7', borderRadius: '4px', padding: '0.625rem 0.875rem', fontSize: '0.8125rem', color: '#065f46' }}>
          {success}
        </div>
      )}
      {error && (
        <div role="alert" style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.625rem 0.875rem', fontSize: '0.8125rem', color: '#dc2626' }}>
          {error}
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
        <label style={{ fontSize: '0.75rem', fontWeight: 600, color: '#374151' }}>Barcode *</label>
        <input
          ref={barcodeRef}
          type="text"
          value={barcode}
          onChange={(e) => setBarcode(e.target.value)}
          placeholder="Scan or enter copy barcode"
          style={{ padding: '0.5rem 0.75rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem' }}
        />
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
        <label style={{ fontSize: '0.75rem', fontWeight: 600, color: '#374151' }}>Reader ID / Number *</label>
        <input
          type="text"
          value={readerId}
          onChange={(e) => setReaderId(e.target.value)}
          placeholder="Reader ID or reader number"
          style={{ padding: '0.5rem 0.75rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem' }}
        />
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
        <label style={{ fontSize: '0.75rem', fontWeight: 600, color: '#374151' }}>Due Date *</label>
        <input
          type="date"
          value={dueDate}
          onChange={(e) => setDueDate(e.target.value)}
          style={{ padding: '0.5rem 0.75rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem' }}
        />
      </div>

      {isAdmin && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
          <label style={{ fontSize: '0.75rem', fontWeight: 600, color: '#374151' }}>Branch ID *</label>
          <input
            type="text"
            value={branchId}
            onChange={(e) => setBranchId(e.target.value)}
            placeholder="Branch UUID (required for administrator)"
            style={{ padding: '0.5rem 0.75rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem', fontFamily: 'monospace' }}
          />
        </div>
      )}

      <button
        type="submit"
        disabled={submitting}
        style={{ padding: '0.625rem 1.25rem', background: '#2563eb', color: '#fff', border: 'none', borderRadius: '4px', cursor: submitting ? 'not-allowed' : 'pointer', fontSize: '0.875rem', fontWeight: 600, opacity: submitting ? 0.6 : 1, alignSelf: 'flex-start' }}
      >
        {submitting ? 'Checking out…' : 'Checkout'}
      </button>
    </form>
  );
}

// ── Return tab ────────────────────────────────────────────────────────────────

function ReturnTab({ isAdmin }: { isAdmin: boolean }) {
  const barcodeRef = useRef<HTMLInputElement>(null);
  const [barcode, setBarcode] = useState('');
  const [branchId, setBranchId] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    barcodeRef.current?.focus();
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const b = barcode.trim();
    if (!b) { setError('Barcode is required'); return; }
    if (isAdmin && !branchId.trim()) { setError('Branch ID is required for administrator accounts'); return; }
    setSubmitting(true);
    setSuccess(null);
    setError(null);
    try {
      await circulationApi.return({
        barcode: b,
        ...(isAdmin && branchId.trim() ? { branch_id: branchId.trim() } : {}),
      });
      setSuccess(`Returned: ${b}`);
      setBarcode('');
      barcodeRef.current?.focus();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Return failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1rem', maxWidth: '480px' }}>
      {success && (
        <div style={{ background: '#d1fae5', border: '1px solid #6ee7b7', borderRadius: '4px', padding: '0.625rem 0.875rem', fontSize: '0.8125rem', color: '#065f46' }}>
          {success}
        </div>
      )}
      {error && (
        <div role="alert" style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.625rem 0.875rem', fontSize: '0.8125rem', color: '#dc2626' }}>
          {error}
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
        <label style={{ fontSize: '0.75rem', fontWeight: 600, color: '#374151' }}>Barcode *</label>
        <input
          ref={barcodeRef}
          type="text"
          value={barcode}
          onChange={(e) => setBarcode(e.target.value)}
          placeholder="Scan or enter copy barcode"
          style={{ padding: '0.5rem 0.75rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem' }}
        />
      </div>

      {isAdmin && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
          <label style={{ fontSize: '0.75rem', fontWeight: 600, color: '#374151' }}>Branch ID *</label>
          <input
            type="text"
            value={branchId}
            onChange={(e) => setBranchId(e.target.value)}
            placeholder="Branch UUID (required for administrator)"
            style={{ padding: '0.5rem 0.75rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem', fontFamily: 'monospace' }}
          />
        </div>
      )}

      <button
        type="submit"
        disabled={submitting}
        style={{ padding: '0.625rem 1.25rem', background: '#065f46', color: '#fff', border: 'none', borderRadius: '4px', cursor: submitting ? 'not-allowed' : 'pointer', fontSize: '0.875rem', fontWeight: 600, opacity: submitting ? 0.6 : 1, alignSelf: 'flex-start' }}
      >
        {submitting ? 'Processing…' : 'Return'}
      </button>
    </form>
  );
}

// ── History tab ───────────────────────────────────────────────────────────────

function HistoryTab() {
  const [result, setResult] = useState<PageResult<CirculationEvent> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const perPage = 20;

  useEffect(() => {
    setLoading(true);
    setError(null);
    circulationApi.list({ page, per_page: perPage })
      .then(setResult)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load history'))
      .finally(() => setLoading(false));
  }, [page]);

  if (loading) return <div style={{ color: '#6b7280', fontSize: '0.8125rem' }}>Loading…</div>;
  if (error) return (
    <div style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.625rem 0.875rem', fontSize: '0.8125rem', color: '#dc2626' }}>
      {error}
    </div>
  );
  if (!result || result.items.length === 0) {
    return <div style={{ color: '#6b7280', fontSize: '0.8125rem' }}>No circulation events found.</div>;
  }

  return (
    <div style={{ overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
        <thead>
          <tr style={{ background: '#f9fafb' }}>
            {['Date', 'Event Type', 'Copy ID', 'Reader ID', 'Branch', 'Due Date', 'Returned'].map((h) => (
              <th key={h} style={{ padding: '0.375rem 0.75rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', color: '#6b7280', borderBottom: '2px solid #e5e7eb', whiteSpace: 'nowrap' }}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {result.items.map((ev) => (
            <tr key={ev.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
              <td style={{ padding: '0.375rem 0.75rem', whiteSpace: 'nowrap' }}>{new Date(ev.created_at).toLocaleString()}</td>
              <td style={{ padding: '0.375rem 0.75rem' }}><EventTypeBadge type={ev.event_type} /></td>
              <td style={{ padding: '0.375rem 0.75rem', fontFamily: 'monospace', fontSize: '0.75rem' }}>{ev.copy_id}</td>
              <td style={{ padding: '0.375rem 0.75rem', fontFamily: 'monospace', fontSize: '0.75rem' }}>{ev.reader_id}</td>
              <td style={{ padding: '0.375rem 0.75rem', fontFamily: 'monospace', fontSize: '0.75rem' }}>{ev.branch_id}</td>
              <td style={{ padding: '0.375rem 0.75rem' }}>{ev.due_date ?? '—'}</td>
              <td style={{ padding: '0.375rem 0.75rem' }}>{ev.returned_at ? new Date(ev.returned_at).toLocaleDateString() : '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {result.total_pages > 1 && (
        <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'space-between', alignItems: 'center', paddingTop: '0.75rem', fontSize: '0.8125rem', color: '#6b7280' }}>
          <span>Page {page} of {result.total_pages} ({result.total.toLocaleString()} events)</span>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button disabled={page <= 1} onClick={() => setPage(page - 1)} style={{ padding: '0.25rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', cursor: page <= 1 ? 'default' : 'pointer', opacity: page <= 1 ? 0.4 : 1, background: '#fff' }}>‹ Prev</button>
            <button disabled={page >= result.total_pages} onClick={() => setPage(page + 1)} style={{ padding: '0.25rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', cursor: page >= result.total_pages ? 'default' : 'pointer', opacity: page >= result.total_pages ? 0.4 : 1, background: '#fff' }}>Next ›</button>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

type Tab = 'checkout' | 'return' | 'history';

export default function CirculationPage() {
  const { hasPermission, hasRole } = useAuth();
  const canWrite = hasPermission('circulation:write');
  const canRead = hasPermission('circulation:read');
  // Administrators have branchID="" on the server; they must supply a branch_id
  // in checkout/return requests so the event can be recorded against a branch.
  const isAdmin = hasRole('administrator');

  const [activeTab, setActiveTab] = useState<Tab>(canWrite ? 'checkout' : 'history');

  const TAB_STYLE = (t: Tab): React.CSSProperties => ({
    padding: '0.5rem 1.25rem',
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
      <div style={{ fontSize: '1.125rem', fontWeight: 700, color: '#1a1a2e', marginBottom: '1rem' }}>Circulation</div>

      {/* Tab navigation */}
      <div style={{ display: 'flex', borderBottom: '1px solid #e5e7eb', marginBottom: '1.5rem' }}>
        {canWrite && <button style={TAB_STYLE('checkout')} onClick={() => setActiveTab('checkout')}>Checkout</button>}
        {canWrite && <button style={TAB_STYLE('return')} onClick={() => setActiveTab('return')}>Return</button>}
        {canRead && <button style={TAB_STYLE('history')} onClick={() => setActiveTab('history')}>History</button>}
      </div>

      {/* Tab content */}
      {activeTab === 'checkout' && canWrite && <CheckoutTab isAdmin={isAdmin} />}
      {activeTab === 'return' && canWrite && <ReturnTab isAdmin={isAdmin} />}
      {activeTab === 'history' && canRead && <HistoryTab />}

      {!canWrite && !canRead && (
        <div style={{ color: '#6b7280', fontSize: '0.875rem' }}>You do not have permission to access circulation.</div>
      )}
    </div>
  );
}
