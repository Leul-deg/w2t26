// StocktakePage — stocktake workstation: session list + active session panel.

import { useEffect, useRef, useState } from 'react';
import { useAuth } from '../../auth/AuthContext';
import { stocktakeApi, StocktakeSession, StocktakeFinding, StocktakeVariance, PageResult } from '../../api/stocktake';

// ── Status badge ──────────────────────────────────────────────────────────────

const SESSION_STATUS_COLOUR: Record<string, { bg: string; color: string }> = {
  open:      { bg: '#d1fae5', color: '#065f46' },
  closed:    { bg: '#f3f4f6', color: '#6b7280' },
  cancelled: { bg: '#fee2e2', color: '#991b1b' },
};

function SessionStatusBadge({ status }: { status: string }) {
  const style = SESSION_STATUS_COLOUR[status] ?? { bg: '#f3f4f6', color: '#374151' };
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
      {status}
    </span>
  );
}

// ── Active session panel ──────────────────────────────────────────────────────

type ActiveTab = 'findings' | 'variances';

interface ActiveSessionPanelProps {
  session: StocktakeSession;
  canWrite: boolean;
  onSessionUpdate: (s: StocktakeSession) => void;
}

function ActiveSessionPanel({ session, canWrite, onSessionUpdate }: ActiveSessionPanelProps) {
  const [barcode, setBarcode] = useState('');
  const [scanning, setScanning] = useState(false);
  const [scanResult, setScanResult] = useState<{ ok: boolean; message: string } | null>(null);
  const [findings, setFindings] = useState<StocktakeFinding[]>([]);
  const [variances, setVariances] = useState<StocktakeVariance[]>([]);
  const [findingsLoading, setFindingsLoading] = useState(false);
  const [variancesLoading, setVariancesLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<ActiveTab>('findings');
  const [closing, setClosing] = useState(false);
  const [closeError, setCloseError] = useState<string | null>(null);

  const barcodeInputRef = useRef<HTMLInputElement>(null);

  // Load recent findings when session changes
  useEffect(() => {
    setFindingsLoading(true);
    stocktakeApi.listFindings(session.id, { per_page: 10 })
      .then((r: PageResult<StocktakeFinding>) => setFindings(r.items))
      .catch(() => {})
      .finally(() => setFindingsLoading(false));
  }, [session.id]);

  // Load variances when tab switches to variances
  useEffect(() => {
    if (activeTab !== 'variances') return;
    setVariancesLoading(true);
    stocktakeApi.getVariances(session.id)
      .then(setVariances)
      .catch(() => {})
      .finally(() => setVariancesLoading(false));
  }, [activeTab, session.id]);

  async function handleScan(e: React.FormEvent) {
    e.preventDefault();
    const b = barcode.trim();
    if (!b) return;
    setScanning(true);
    setScanResult(null);
    try {
      const finding = await stocktakeApi.recordScan(session.id, b);
      setScanResult({ ok: true, message: `Scanned: ${finding.barcode ?? b} (${finding.finding_type})` });
      setBarcode('');
      // Prepend to findings list, keep max 10
      setFindings((prev) => [finding, ...prev].slice(0, 10));
    } catch (err) {
      setScanResult({ ok: false, message: err instanceof Error ? err.message : 'Scan failed' });
    } finally {
      setScanning(false);
      barcodeInputRef.current?.focus();
    }
  }

  async function handleClose(status: 'closed' | 'cancelled') {
    const label = status === 'closed' ? 'Close' : 'Cancel';
    if (!window.confirm(`${label} this session?`)) return;
    setClosing(true);
    setCloseError(null);
    try {
      const updated = await stocktakeApi.close(session.id, status);
      onSessionUpdate(updated);
    } catch (err) {
      setCloseError(err instanceof Error ? err.message : `Failed to ${label.toLowerCase()} session`);
    } finally {
      setClosing(false);
    }
  }

  const isOpen = session.status === 'open';

  const TAB_STYLE = (t: ActiveTab): React.CSSProperties => ({
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
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      {/* Session header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', flexWrap: 'wrap', gap: '0.5rem' }}>
        <div>
          <div style={{ fontSize: '1.125rem', fontWeight: 700, color: '#1a1a2e' }}>{session.name}</div>
          <div style={{ fontSize: '0.8125rem', color: '#6b7280', marginTop: '0.25rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <SessionStatusBadge status={session.status} />
            <span>Started {new Date(session.started_at).toLocaleString()}</span>
          </div>
        </div>
        {canWrite && isOpen && (
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button
              onClick={() => handleClose('closed')}
              disabled={closing}
              style={{ padding: '0.375rem 0.75rem', background: '#065f46', color: '#fff', border: 'none', borderRadius: '4px', cursor: closing ? 'not-allowed' : 'pointer', fontSize: '0.8125rem', opacity: closing ? 0.6 : 1 }}
            >
              Close Session
            </button>
            <button
              onClick={() => handleClose('cancelled')}
              disabled={closing}
              style={{ padding: '0.375rem 0.75rem', background: '#fff', color: '#dc2626', border: '1px solid #fca5a5', borderRadius: '4px', cursor: closing ? 'not-allowed' : 'pointer', fontSize: '0.8125rem', opacity: closing ? 0.6 : 1 }}
            >
              Cancel Session
            </button>
          </div>
        )}
      </div>

      {closeError && (
        <div style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.5rem 0.75rem', fontSize: '0.8125rem', color: '#dc2626' }}>
          {closeError}
        </div>
      )}

      {/* Scan form — only when open */}
      {isOpen && canWrite && (
        <div style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: '6px', padding: '1rem' }}>
          <div style={{ fontSize: '0.8125rem', fontWeight: 600, color: '#374151', marginBottom: '0.625rem' }}>Scan Barcode</div>
          <form onSubmit={handleScan} style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
            <input
              ref={barcodeInputRef}
              type="text"
              value={barcode}
              onChange={(e) => setBarcode(e.target.value)}
              placeholder="Scan or enter barcode…"
              autoFocus
              style={{ flex: 1, padding: '0.5rem 0.75rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem' }}
            />
            <button
              type="submit"
              disabled={scanning || !barcode.trim()}
              style={{ padding: '0.5rem 1rem', background: '#2563eb', color: '#fff', border: 'none', borderRadius: '4px', cursor: scanning ? 'not-allowed' : 'pointer', fontSize: '0.875rem', fontWeight: 600, opacity: scanning || !barcode.trim() ? 0.6 : 1 }}
            >
              {scanning ? 'Scanning…' : 'Scan'}
            </button>
          </form>
          {scanResult && (
            <div style={{
              marginTop: '0.5rem',
              padding: '0.375rem 0.625rem',
              background: scanResult.ok ? '#d1fae5' : '#fef2f2',
              color: scanResult.ok ? '#065f46' : '#dc2626',
              borderRadius: '4px',
              fontSize: '0.8125rem',
            }}>
              {scanResult.message}
            </div>
          )}
        </div>
      )}

      {/* Tabs */}
      <div style={{ borderBottom: '1px solid #e5e7eb' }}>
        <button style={TAB_STYLE('findings')} onClick={() => setActiveTab('findings')}>Recent Findings</button>
        <button style={TAB_STYLE('variances')} onClick={() => setActiveTab('variances')}>Variances</button>
      </div>

      {/* Findings tab */}
      {activeTab === 'findings' && (
        <div>
          {findingsLoading ? (
            <div style={{ color: '#6b7280', fontSize: '0.8125rem' }}>Loading…</div>
          ) : findings.length === 0 ? (
            <div style={{ color: '#6b7280', fontSize: '0.8125rem' }}>No findings yet.</div>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr style={{ background: '#f9fafb' }}>
                  {['Time', 'Barcode', 'Type', 'Notes'].map((h) => (
                    <th key={h} style={{ padding: '0.375rem 0.75rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', color: '#6b7280', borderBottom: '2px solid #e5e7eb' }}>
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {findings.map((f) => (
                  <tr key={f.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                    <td style={{ padding: '0.375rem 0.75rem', whiteSpace: 'nowrap' }}>{new Date(f.created_at).toLocaleTimeString()}</td>
                    <td style={{ padding: '0.375rem 0.75rem', fontFamily: 'monospace' }}>{f.barcode ?? '—'}</td>
                    <td style={{ padding: '0.375rem 0.75rem' }}>{f.finding_type}</td>
                    <td style={{ padding: '0.375rem 0.75rem', color: f.notes ? '#1a1a2e' : '#9ca3af', fontStyle: f.notes ? undefined : 'italic' }}>{f.notes ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* Variances tab */}
      {activeTab === 'variances' && (
        <div>
          {variancesLoading ? (
            <div style={{ color: '#6b7280', fontSize: '0.8125rem' }}>Loading…</div>
          ) : variances.length === 0 ? (
            <div style={{ color: '#6b7280', fontSize: '0.8125rem' }}>No variances found.</div>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr style={{ background: '#f9fafb' }}>
                  {['Barcode', 'Expected Status', 'Found Status', 'Finding Type'].map((h) => (
                    <th key={h} style={{ padding: '0.375rem 0.75rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', color: '#6b7280', borderBottom: '2px solid #e5e7eb' }}>
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {variances.map((v) => (
                  <tr key={v.copy_id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                    <td style={{ padding: '0.375rem 0.75rem', fontFamily: 'monospace' }}>{v.barcode}</td>
                    <td style={{ padding: '0.375rem 0.75rem' }}>{v.expected_status}</td>
                    <td style={{ padding: '0.375rem 0.75rem' }}>{v.found_status}</td>
                    <td style={{ padding: '0.375rem 0.75rem' }}>{v.finding_type}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function StocktakePage() {
  const { hasPermission } = useAuth();
  const canWrite = hasPermission('stocktake:write');

  const [sessions, setSessions] = useState<StocktakeSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(true);
  const [activeSession, setActiveSession] = useState<StocktakeSession | null>(null);
  const [newName, setNewName] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  useEffect(() => {
    setSessionsLoading(true);
    stocktakeApi.list({ per_page: 50 })
      .then((r) => setSessions(r.items))
      .catch(() => {})
      .finally(() => setSessionsLoading(false));
  }, []);

  async function handleCreateSession(e: React.FormEvent) {
    e.preventDefault();
    const name = newName.trim();
    if (!name) return;
    setCreating(true);
    setCreateError(null);
    try {
      const session = await stocktakeApi.create({ name });
      setSessions((prev) => [session, ...prev]);
      setActiveSession(session);
      setNewName('');
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'Failed to create session');
    } finally {
      setCreating(false);
    }
  }

  function handleSessionUpdate(updated: StocktakeSession) {
    setSessions((prev) => prev.map((s) => s.id === updated.id ? updated : s));
    setActiveSession(updated);
  }

  return (
    <div>
      <div style={{ fontSize: '1.125rem', fontWeight: 700, color: '#1a1a2e', marginBottom: '1rem' }}>Stocktake</div>

      <div style={{ display: 'grid', gridTemplateColumns: '280px 1fr', gap: '1rem', alignItems: 'start' }}>
        {/* Left panel — session list */}
        <div style={{ border: '1px solid #e5e7eb', borderRadius: '6px', overflow: 'hidden', background: '#fff' }}>
          <div style={{ padding: '0.625rem 1rem', borderBottom: '1px solid #e5e7eb', background: '#f9fafb', fontWeight: 600, fontSize: '0.8125rem', color: '#374151' }}>
            Sessions
          </div>

          {canWrite && (
            <div style={{ padding: '0.75rem', borderBottom: '1px solid #f3f4f6' }}>
              <form onSubmit={handleCreateSession} style={{ display: 'flex', flexDirection: 'column', gap: '0.375rem' }}>
                <input
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="New session name…"
                  style={{ padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.8125rem', width: '100%', boxSizing: 'border-box' }}
                />
                {createError && (
                  <div style={{ fontSize: '0.75rem', color: '#dc2626' }}>{createError}</div>
                )}
                <button
                  type="submit"
                  disabled={creating || !newName.trim()}
                  style={{ padding: '0.375rem 0.625rem', background: '#2563eb', color: '#fff', border: 'none', borderRadius: '4px', cursor: creating ? 'not-allowed' : 'pointer', fontSize: '0.8125rem', fontWeight: 600, opacity: creating || !newName.trim() ? 0.6 : 1 }}
                >
                  {creating ? 'Creating…' : '+ New Session'}
                </button>
              </form>
            </div>
          )}

          <div style={{ maxHeight: '60vh', overflowY: 'auto' }}>
            {sessionsLoading ? (
              <div style={{ padding: '1rem', color: '#6b7280', fontSize: '0.8125rem', textAlign: 'center' }}>Loading…</div>
            ) : sessions.length === 0 ? (
              <div style={{ padding: '1rem', color: '#6b7280', fontSize: '0.8125rem', textAlign: 'center' }}>No sessions yet.</div>
            ) : (
              sessions.map((s) => (
                <button
                  key={s.id}
                  onClick={() => setActiveSession(s)}
                  style={{
                    display: 'block',
                    width: '100%',
                    textAlign: 'left',
                    padding: '0.625rem 0.875rem',
                    background: activeSession?.id === s.id ? '#eff6ff' : 'none',
                    border: 'none',
                    borderBottom: '1px solid #f3f4f6',
                    cursor: 'pointer',
                    borderLeft: activeSession?.id === s.id ? '3px solid #2563eb' : '3px solid transparent',
                  }}
                >
                  <div style={{ fontSize: '0.8125rem', fontWeight: 600, color: '#1a1a2e', marginBottom: '0.25rem' }}>{s.name}</div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.375rem' }}>
                    <SessionStatusBadge status={s.status} />
                    <span style={{ fontSize: '0.6875rem', color: '#6b7280' }}>{new Date(s.started_at).toLocaleDateString()}</span>
                  </div>
                </button>
              ))
            )}
          </div>
        </div>

        {/* Right panel — active session */}
        <div style={{ border: '1px solid #e5e7eb', borderRadius: '6px', padding: '1rem', background: '#fff', minHeight: '200px' }}>
          {activeSession ? (
            <ActiveSessionPanel
              key={activeSession.id}
              session={activeSession}
              canWrite={canWrite}
              onSessionUpdate={handleSessionUpdate}
            />
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '200px', color: '#6b7280', fontSize: '0.875rem' }}>
              Select a session to begin
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
