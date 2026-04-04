// HoldingDetailPage — detail view for a single holding with its copies.

import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { holdingsApi, Holding, Copy, CopyStatus, PageResult } from '../../api/holdings';
import { HttpError } from '../../api/client';

// ── Badges ────────────────────────────────────────────────────────────────────

function ActiveBadge({ active }: { active: boolean }) {
  return (
    <span style={{
      display: 'inline-block',
      padding: '0.1875rem 0.625rem',
      borderRadius: '9999px',
      fontSize: '0.75rem',
      fontWeight: 600,
      background: active ? '#d1fae5' : '#f3f4f6',
      color: active ? '#065f46' : '#6b7280',
    }}>
      {active ? 'Active' : 'Inactive'}
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

// ── Add Copy modal ────────────────────────────────────────────────────────────

interface AddCopyModalProps {
  holdingId: string;
  statuses: CopyStatus[];
  onAdded: (copy: Copy) => void;
  onCancel: () => void;
}

function AddCopyModal({ holdingId, statuses, onAdded, onCancel }: AddCopyModalProps) {
  const [barcode, setBarcode] = useState('');
  const [condition, setCondition] = useState('good');
  const [statusCode, setStatusCode] = useState(statuses[0]?.code ?? 'available');
  const [shelfLocation, setShelfLocation] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!barcode.trim()) { setError('Barcode is required'); return; }
    setSubmitting(true);
    setError(null);
    try {
      const copy = await holdingsApi.addCopy(holdingId, {
        barcode: barcode.trim(),
        condition,
        status_code: statusCode,
        shelf_location: shelfLocation.trim() || undefined,
      });
      onAdded(copy);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add copy');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 10000 }}
      onClick={(e) => { if (e.target === e.currentTarget) onCancel(); }}
    >
      <div style={{ background: '#fff', borderRadius: '6px', padding: '1.5rem', width: '100%', maxWidth: '400px', boxShadow: '0 8px 32px rgba(0,0,0,0.18)' }}>
        <div style={{ fontWeight: 700, fontSize: '1rem', marginBottom: '1rem', color: '#1a1a2e' }}>Add Copy</div>
        {error && (
          <div style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.5rem 0.75rem', fontSize: '0.8125rem', color: '#dc2626', marginBottom: '0.75rem' }}>
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          <div>
            <label style={{ display: 'block', fontSize: '0.75rem', fontWeight: 600, color: '#374151', marginBottom: '0.25rem' }}>Barcode *</label>
            <input
              type="text"
              value={barcode}
              onChange={(e) => setBarcode(e.target.value)}
              autoFocus
              style={{ width: '100%', padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem', boxSizing: 'border-box' }}
            />
          </div>
          <div>
            <label style={{ display: 'block', fontSize: '0.75rem', fontWeight: 600, color: '#374151', marginBottom: '0.25rem' }}>Status</label>
            <select
              value={statusCode}
              onChange={(e) => setStatusCode(e.target.value)}
              style={{ width: '100%', padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem' }}
            >
              {statuses.map((s) => (
                <option key={s.code} value={s.code}>{s.description}</option>
              ))}
            </select>
          </div>
          <div>
            <label style={{ display: 'block', fontSize: '0.75rem', fontWeight: 600, color: '#374151', marginBottom: '0.25rem' }}>Condition</label>
            <select
              value={condition}
              onChange={(e) => setCondition(e.target.value)}
              style={{ width: '100%', padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem' }}
            >
              <option value="new">New</option>
              <option value="good">Good</option>
              <option value="fair">Fair</option>
              <option value="poor">Poor</option>
            </select>
          </div>
          <div>
            <label style={{ display: 'block', fontSize: '0.75rem', fontWeight: 600, color: '#374151', marginBottom: '0.25rem' }}>Shelf Location</label>
            <input
              type="text"
              value={shelfLocation}
              onChange={(e) => setShelfLocation(e.target.value)}
              placeholder="e.g. A3-S2"
              style={{ width: '100%', padding: '0.375rem 0.625rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.875rem', boxSizing: 'border-box' }}
            />
          </div>
          <div style={{ display: 'flex', gap: '0.625rem', justifyContent: 'flex-end', paddingTop: '0.25rem' }}>
            <button type="button" onClick={onCancel} style={{ padding: '0.5rem 1rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: '4px', cursor: 'pointer', fontSize: '0.875rem' }}>
              Cancel
            </button>
            <button type="submit" disabled={submitting} style={{ padding: '0.5rem 1rem', background: '#2563eb', color: '#fff', border: 'none', borderRadius: '4px', cursor: submitting ? 'not-allowed' : 'pointer', fontSize: '0.875rem', fontWeight: 600, opacity: submitting ? 0.6 : 1 }}>
              {submitting ? 'Adding…' : 'Add Copy'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Copies table ──────────────────────────────────────────────────────────────

interface CopiesTableProps {
  copies: Copy[];
  statuses: CopyStatus[];
  canWrite: boolean;
  onStatusChange: (copyId: string, newStatus: string) => Promise<void>;
  onAddCopy: () => void;
  canAddCopy: boolean;
}

function CopiesTable({ copies, statuses, canWrite, onStatusChange, onAddCopy, canAddCopy }: CopiesTableProps) {
  const [updatingId, setUpdatingId] = useState<string | null>(null);

  async function handleStatusChange(copyId: string, newStatus: string) {
    setUpdatingId(copyId);
    await onStatusChange(copyId, newStatus);
    setUpdatingId(null);
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '0.625rem' }}>
        {canAddCopy && (
          <button
            onClick={onAddCopy}
            style={{ padding: '0.375rem 0.875rem', background: '#2563eb', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '0.8125rem', fontWeight: 600 }}
          >
            + Add Copy
          </button>
        )}
      </div>
      {copies.length === 0 ? (
        <div style={{ color: '#6b7280', fontSize: '0.8125rem', padding: '0.5rem 0' }}>No copies yet.</div>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
          <thead>
            <tr style={{ background: '#f9fafb' }}>
              {['Barcode', 'Status', 'Condition', 'Shelf Location', ...(canWrite ? ['Update Status'] : [])].map((h) => (
                <th key={h} style={{ padding: '0.375rem 0.75rem', textAlign: 'left', fontWeight: 600, fontSize: '0.75rem', color: '#6b7280', borderBottom: '2px solid #e5e7eb' }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {copies.map((c) => (
              <tr key={c.id} style={{ borderBottom: '1px solid #f3f4f6' }}>
                <td style={{ padding: '0.375rem 0.75rem', fontFamily: 'monospace' }}>{c.barcode}</td>
                <td style={{ padding: '0.375rem 0.75rem' }}>{c.status_code}</td>
                <td style={{ padding: '0.375rem 0.75rem', textTransform: 'capitalize' }}>{c.condition}</td>
                <td style={{ padding: '0.375rem 0.75rem', color: c.shelf_location ? '#1a1a2e' : '#9ca3af', fontStyle: c.shelf_location ? undefined : 'italic' }}>
                  {c.shelf_location ?? '—'}
                </td>
                {canWrite && (
                  <td style={{ padding: '0.375rem 0.75rem' }}>
                    <select
                      value={c.status_code}
                      disabled={updatingId === c.id}
                      onChange={(e) => handleStatusChange(c.id, e.target.value)}
                      style={{ padding: '0.25rem 0.5rem', border: '1px solid #d1d5db', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
                    >
                      {statuses.map((s) => (
                        <option key={s.code} value={s.code}>{s.description}</option>
                      ))}
                    </select>
                    {updatingId === c.id && <span style={{ marginLeft: '0.5rem', color: '#6b7280', fontSize: '0.75rem' }}>…</span>}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function HoldingDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { hasPermission } = useAuth();

  const canWrite = hasPermission('holdings:write');
  const canAddCopy = hasPermission('copies:write');

  const [holding, setHolding] = useState<Holding | null>(null);
  const [copiesResult, setCopiesResult] = useState<PageResult<Copy> | null>(null);
  const [statuses, setStatuses] = useState<CopyStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copyError, setCopyError] = useState<string | null>(null);
  const [showAddCopy, setShowAddCopy] = useState(false);
  const [deactivating, setDeactivating] = useState(false);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    Promise.all([
      holdingsApi.get(id),
      holdingsApi.listCopies(id),
      holdingsApi.listCopyStatuses(),
    ])
      .then(([h, copies, st]) => {
        setHolding(h);
        setCopiesResult(copies);
        setStatuses(st);
      })
      .catch((err) => {
        if (err instanceof HttpError && err.status === 404) {
          setError('Holding not found.');
        } else {
          setError(err instanceof Error ? err.message : 'Failed to load holding');
        }
      })
      .finally(() => setLoading(false));
  }, [id]);

  async function handleDeactivate() {
    if (!id || !holding) return;
    if (!window.confirm(`Deactivate "${holding.title}"? This will mark it as inactive.`)) return;
    setDeactivating(true);
    try {
      await holdingsApi.deactivate(id);
      setHolding((h) => h ? { ...h, is_active: false } : h);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to deactivate holding');
    } finally {
      setDeactivating(false);
    }
  }

  async function handleCopyStatusChange(copyId: string, newStatus: string) {
    setCopyError(null);
    try {
      const updated = await holdingsApi.updateCopyStatus(copyId, newStatus);
      setCopiesResult((r) => r ? {
        ...r,
        items: r.items.map((c) => c.id === copyId ? updated : c),
      } : r);
    } catch (err) {
      setCopyError(err instanceof Error ? err.message : 'Failed to update copy status');
    }
  }

  function handleCopyAdded(copy: Copy) {
    setCopiesResult((r) => r ? {
      ...r,
      items: [...r.items, copy],
      total: r.total + 1,
    } : { items: [copy], total: 1, page: 1, per_page: 20, total_pages: 1 });
    setShowAddCopy(false);
  }

  if (loading) {
    return <div style={{ padding: '2rem', textAlign: 'center', color: '#6b7280' }}>Loading…</div>;
  }

  if (error || !holding) {
    return (
      <div>
        <button onClick={() => navigate('/holdings')} style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', fontSize: '0.8125rem', padding: '0 0 0.75rem 0' }}>
          ← Back to holdings
        </button>
        <div style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.75rem 1rem', color: '#dc2626', fontSize: '0.8125rem' }}>
          {error ?? 'Holding not found'}
        </div>
      </div>
    );
  }

  return (
    <div>
      {/* Back link */}
      <button
        onClick={() => navigate('/holdings')}
        style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', fontSize: '0.8125rem', padding: '0 0 0.75rem 0' }}
      >
        ← Back to holdings
      </button>

      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '1rem', flexWrap: 'wrap', gap: '0.5rem' }}>
        <div>
          <div style={{ fontSize: '1.25rem', fontWeight: 700, color: '#1a1a2e' }}>{holding.title}</div>
          {holding.author && (
            <div style={{ fontSize: '0.875rem', color: '#6b7280', marginTop: '0.25rem' }}>by {holding.author}</div>
          )}
        </div>
        {canWrite && (
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <button
              onClick={() => navigate(`/holdings/${holding.id}/edit`)}
              style={{ padding: '0.4375rem 0.875rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: '4px', cursor: 'pointer', fontSize: '0.8125rem' }}
            >
              Edit
            </button>
            {holding.is_active && (
              <button
                onClick={handleDeactivate}
                disabled={deactivating}
                style={{ padding: '0.4375rem 0.875rem', background: '#fff', color: '#dc2626', border: '1px solid #fca5a5', borderRadius: '4px', cursor: deactivating ? 'not-allowed' : 'pointer', fontSize: '0.8125rem', opacity: deactivating ? 0.6 : 1 }}
              >
                {deactivating ? 'Deactivating…' : 'Deactivate'}
              </button>
            )}
          </div>
        )}
      </div>

      {/* Metadata */}
      <SectionCard title="Details">
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.5rem' }}>
          <ActiveBadge active={holding.is_active} />
        </div>
        <FieldRow label="ISBN" value={holding.isbn} />
        <FieldRow label="Publisher" value={holding.publisher} />
        <FieldRow label="Year" value={holding.publication_year?.toString()} />
        <FieldRow label="Category" value={holding.category} />
        <FieldRow label="Subcategory" value={holding.subcategory} />
        <FieldRow label="Language" value={holding.language} />
        {holding.description && <FieldRow label="Description" value={holding.description} />}
      </SectionCard>

      {/* Copies */}
      {copyError && (
        <div style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.5rem 0.75rem', fontSize: '0.8125rem', color: '#dc2626', marginBottom: '0.75rem' }}>
          {copyError}
        </div>
      )}
      <SectionCard title={`Copies (${copiesResult?.total ?? 0})`}>
        <CopiesTable
          copies={copiesResult?.items ?? []}
          statuses={statuses}
          canWrite={canWrite}
          onStatusChange={handleCopyStatusChange}
          onAddCopy={() => setShowAddCopy(true)}
          canAddCopy={canAddCopy}
        />
      </SectionCard>

      {/* Add Copy modal */}
      {showAddCopy && (
        <AddCopyModal
          holdingId={holding.id}
          statuses={statuses}
          onAdded={handleCopyAdded}
          onCancel={() => setShowAddCopy(false)}
        />
      )}
    </div>
  );
}
