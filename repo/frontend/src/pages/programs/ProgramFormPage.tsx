// ProgramFormPage — create or edit a program.
//
// Create: navigates to /programs/new
// Edit:   navigates to /programs/:id/edit — pre-populates all fields.
//
// Requires programs:write permission.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { programsApi } from '../../api/programs';

function toLocalDatetimeValue(iso: string): string {
  // Convert ISO 8601 UTC string to the datetime-local input value (YYYY-MM-DDTHH:mm).
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function fromLocalDatetimeValue(local: string): string {
  // Convert datetime-local input value to ISO 8601 string (treating as local time).
  return new Date(local).toISOString();
}

const INPUT_STYLE: React.CSSProperties = {
  padding: '0.375rem 0.625rem',
  border: '1px solid #d1d5db',
  borderRadius: 6,
  fontSize: '0.875rem',
  width: '100%',
  boxSizing: 'border-box',
};

const LABEL_STYLE: React.CSSProperties = {
  display: 'block',
  fontSize: '0.8125rem',
  fontWeight: 600,
  color: '#374151',
  marginBottom: '0.25rem',
};

const HINT_STYLE: React.CSSProperties = {
  fontSize: '0.75rem',
  color: '#9ca3af',
  marginTop: '0.125rem',
};

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: '1rem' }}>
      <label style={LABEL_STYLE}>{label}</label>
      {children}
      {hint && <p style={HINT_STYLE}>{hint}</p>}
    </div>
  );
}

export default function ProgramFormPage() {
  const { id } = useParams<{ id: string }>();
  const isEdit = Boolean(id);
  const navigate = useNavigate();
  const { hasPermission, hasRole } = useAuth();
  const canWrite = hasPermission('programs:write');
  const isAdmin = hasRole('administrator');

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [category, setCategory] = useState('');
  const [venueType, setVenueType] = useState('');
  const [venueName, setVenueName] = useState('');
  const [branchId, setBranchId] = useState('');
  const [capacity, setCapacity] = useState('20');
  const [startsAt, setStartsAt] = useState('');
  const [endsAt, setEndsAt] = useState('');
  const [enrollmentOpensAt, setEnrollmentOpensAt] = useState('');
  const [enrollmentClosesAt, setEnrollmentClosesAt] = useState('');
  const [enrollmentChannel, setEnrollmentChannel] = useState('any');

  const [loading, setLoading] = useState(isEdit);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchProgram = useCallback(async () => {
    if (!id) return;
    try {
      const p = await programsApi.get(id);
      setTitle(p.title);
      setDescription(p.description ?? '');
      setCategory(p.category ?? '');
      setVenueType(p.venue_type ?? '');
      setVenueName(p.venue_name ?? '');
      setCapacity(String(p.capacity));
      setStartsAt(toLocalDatetimeValue(p.starts_at));
      setEndsAt(toLocalDatetimeValue(p.ends_at));
      setEnrollmentOpensAt(p.enrollment_opens_at ? toLocalDatetimeValue(p.enrollment_opens_at) : '');
      setEnrollmentClosesAt(p.enrollment_closes_at ? toLocalDatetimeValue(p.enrollment_closes_at) : '');
      setEnrollmentChannel(p.enrollment_channel);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load program');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    if (isEdit) fetchProgram();
  }, [isEdit, fetchProgram]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!title.trim()) { setError('Title is required'); return; }
    if (!startsAt) { setError('Start date/time is required'); return; }
    if (!endsAt) { setError('End date/time is required'); return; }
    if (!isEdit && isAdmin && !branchId.trim()) { setError('Branch ID is required for administrator-created programs'); return; }
    const cap = parseInt(capacity, 10);
    if (isNaN(cap) || cap < 1) { setError('Capacity must be a positive integer'); return; }

    const payload = {
      ...(!isEdit && isAdmin && branchId.trim() ? { branch_id: branchId.trim() } : {}),
      title: title.trim(),
      description: description.trim() || undefined,
      category: category.trim() || undefined,
      venue_type: venueType.trim() || undefined,
      venue_name: venueName.trim() || undefined,
      capacity: cap,
      starts_at: fromLocalDatetimeValue(startsAt),
      ends_at: fromLocalDatetimeValue(endsAt),
      enrollment_opens_at: enrollmentOpensAt ? fromLocalDatetimeValue(enrollmentOpensAt) : undefined,
      enrollment_closes_at: enrollmentClosesAt ? fromLocalDatetimeValue(enrollmentClosesAt) : undefined,
      enrollment_channel: enrollmentChannel,
    };

    setSaving(true);
    try {
      if (isEdit && id) {
        await programsApi.update(id, payload);
        navigate(`/programs/${id}`);
      } else {
        const created = await programsApi.create(payload);
        navigate(`/programs/${created.id}`);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  if (!canWrite) {
    return <p style={{ padding: '1.5rem', color: '#dc2626' }}>You do not have permission to manage programs.</p>;
  }

  if (loading) return <p style={{ padding: '1.5rem', color: '#6b7280' }}>Loading…</p>;

  return (
    <div style={{ maxWidth: 680, margin: '0 auto', padding: '1.5rem' }}>
      {/* Header */}
      <div style={{ marginBottom: '1.5rem' }}>
        <button
          onClick={() => navigate(isEdit && id ? `/programs/${id}` : '/programs')}
          style={{ fontSize: '0.8125rem', color: '#6b7280', background: 'none', border: 'none', cursor: 'pointer', marginBottom: 8 }}
        >
          ← {isEdit ? 'Back to program' : 'Programs'}
        </button>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>
          {isEdit ? 'Edit program' : 'New program'}
        </h1>
      </div>

      {error && (
        <div style={{ background: '#fef2f2', border: '1px solid #fecaca', borderRadius: 6, padding: '0.75rem', marginBottom: '1rem' }}>
          <p style={{ margin: 0, color: '#991b1b', fontSize: '0.875rem' }}>{error}</p>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        {!isEdit && isAdmin && (
          <Field label="Branch ID *" hint="Administrator accounts must target a specific branch when creating a program.">
            <input
              value={branchId}
              onChange={(e) => setBranchId(e.target.value)}
              style={{ ...INPUT_STYLE, fontFamily: 'monospace' }}
              placeholder="Branch UUID"
              required
            />
          </Field>
        )}

        <Field label="Title *">
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            style={INPUT_STYLE}
            placeholder="e.g. Summer Reading Club"
            required
          />
        </Field>

        <Field label="Description">
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            style={{ ...INPUT_STYLE, minHeight: 80, resize: 'vertical' }}
            placeholder="Optional description shown to staff on the detail page"
          />
        </Field>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
          <Field label="Category">
            <input
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              style={INPUT_STYLE}
              placeholder="e.g. Youth, Adult"
            />
          </Field>

          <Field label="Capacity *" hint="Total confirmed seats allowed">
            <input
              type="number"
              min={1}
              value={capacity}
              onChange={(e) => setCapacity(e.target.value)}
              style={INPUT_STYLE}
              required
            />
          </Field>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
          <Field label="Venue type">
            <input
              value={venueType}
              onChange={(e) => setVenueType(e.target.value)}
              style={INPUT_STYLE}
              placeholder="e.g. in_person, online"
            />
          </Field>

          <Field label="Venue name">
            <input
              value={venueName}
              onChange={(e) => setVenueName(e.target.value)}
              style={INPUT_STYLE}
              placeholder="e.g. Main Hall"
            />
          </Field>
        </div>

        <div style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1rem', marginBottom: '1rem' }}>
          <p style={{ margin: '0 0 0.75rem', fontWeight: 600, fontSize: '0.875rem' }}>Schedule</p>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <Field label="Starts at *">
              <input
                type="datetime-local"
                value={startsAt}
                onChange={(e) => setStartsAt(e.target.value)}
                style={INPUT_STYLE}
                required
              />
            </Field>

            <Field label="Ends at *">
              <input
                type="datetime-local"
                value={endsAt}
                onChange={(e) => setEndsAt(e.target.value)}
                style={INPUT_STYLE}
                required
              />
            </Field>
          </div>
        </div>

        <div style={{ background: '#f9fafb', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1rem', marginBottom: '1.5rem' }}>
          <p style={{ margin: '0 0 0.75rem', fontWeight: 600, fontSize: '0.875rem' }}>Enrollment window</p>
          <p style={{ margin: '0 0 0.75rem', fontSize: '0.75rem', color: '#6b7280' }}>
            Leave blank to allow enrollment at any time while the program is published.
          </p>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <Field label="Enrollment opens at">
              <input
                type="datetime-local"
                value={enrollmentOpensAt}
                onChange={(e) => setEnrollmentOpensAt(e.target.value)}
                style={INPUT_STYLE}
              />
            </Field>

            <Field label="Enrollment closes at">
              <input
                type="datetime-local"
                value={enrollmentClosesAt}
                onChange={(e) => setEnrollmentClosesAt(e.target.value)}
                style={INPUT_STYLE}
              />
            </Field>
          </div>

          <Field label="Enrollment channel" hint="Controls who can enroll">
            <select
              value={enrollmentChannel}
              onChange={(e) => setEnrollmentChannel(e.target.value)}
              style={INPUT_STYLE}
            >
              <option value="any">Any (self-service + staff)</option>
              <option value="staff_only">Staff only</option>
              <option value="self_service">Self-service only</option>
            </select>
          </Field>
        </div>

        <div style={{ display: 'flex', gap: '0.75rem' }}>
          <button
            type="submit"
            disabled={saving}
            style={{
              padding: '0.5rem 1.25rem',
              background: saving ? '#a5b4fc' : '#4f46e5',
              color: '#fff',
              border: 'none',
              borderRadius: 6,
              fontSize: '0.875rem',
              cursor: saving ? 'not-allowed' : 'pointer',
              fontWeight: 600,
            }}
          >
            {saving ? 'Saving…' : (isEdit ? 'Save changes' : 'Create program')}
          </button>
          <button
            type="button"
            onClick={() => navigate(isEdit && id ? `/programs/${id}` : '/programs')}
            style={{ padding: '0.5rem 1rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
