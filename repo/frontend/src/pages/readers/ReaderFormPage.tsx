// ReaderFormPage — create and edit reader profiles.
//
// Used for both POST /readers (new) and PATCH /readers/:id (edit).
// Sensitive fields (national_id, contact_email, etc.) are encrypted server-side
// before storage — the form sends plaintext which is encrypted in transit via TLS
// and at rest by the backend.

import { FormEvent, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { readersApi, CreateReaderRequest, UpdateReaderRequest } from '../../api/readers';
import LoadingState from '../../components/LoadingState';
import { HttpError } from '../../api/client';

interface FieldState {
  first_name: string;
  last_name: string;
  preferred_name: string;
  notes: string;
  national_id: string;
  contact_email: string;
  contact_phone: string;
  date_of_birth: string;
}

const EMPTY: FieldState = {
  first_name: '',
  last_name: '',
  preferred_name: '',
  notes: '',
  national_id: '',
  contact_email: '',
  contact_phone: '',
  date_of_birth: '',
};

const INPUT_STYLE: React.CSSProperties = {
  display: 'block',
  width: '100%',
  padding: '0.5rem 0.75rem',
  border: '1px solid #d1d5db',
  borderRadius: '4px',
  fontSize: '0.875rem',
  color: '#1a1a2e',
  boxSizing: 'border-box',
};

const LABEL_STYLE: React.CSSProperties = {
  display: 'block',
  fontSize: '0.8125rem',
  fontWeight: 500,
  color: '#374151',
  marginBottom: '0.25rem',
};

function Field({
  label,
  name,
  value,
  onChange,
  type = 'text',
  placeholder,
  required,
  hint,
}: {
  label: string;
  name: keyof FieldState;
  value: string;
  onChange: (name: keyof FieldState, value: string) => void;
  type?: string;
  placeholder?: string;
  required?: boolean;
  hint?: string;
}) {
  return (
    <div>
      <label htmlFor={`field-${name}`} style={LABEL_STYLE}>
        {label}{required && <span style={{ color: '#dc2626' }}> *</span>}
      </label>
      <input
        id={`field-${name}`}
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(name, e.target.value)}
        required={required}
        style={INPUT_STYLE}
      />
      {hint && (
        <div style={{ fontSize: '0.75rem', color: '#6b7280', marginTop: '0.25rem' }}>{hint}</div>
      )}
    </div>
  );
}

export default function ReaderFormPage() {
  const { id } = useParams<{ id?: string }>();
  const navigate = useNavigate();
  const isEdit = Boolean(id);

  const [fields, setFields] = useState<FieldState>(EMPTY);
  const [loadingInit, setLoadingInit] = useState(isEdit);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<keyof FieldState, string>>>({});

  // Load existing reader data when editing.
  useEffect(() => {
    if (!isEdit || !id) return;
    readersApi.get(id).then((r) => {
      setFields({
        first_name: r.first_name,
        last_name: r.last_name,
        preferred_name: r.preferred_name ?? '',
        notes: r.notes ?? '',
        national_id: '',     // never pre-filled — user must re-enter if changing
        contact_email: '',
        contact_phone: '',
        date_of_birth: '',
      });
    }).catch((err) => {
      setError(err instanceof Error ? err.message : 'Failed to load reader');
    }).finally(() => setLoadingInit(false));
  }, [id, isEdit]);

  function handleChange(name: keyof FieldState, value: string) {
    setFields((f) => ({ ...f, [name]: value }));
    setFieldErrors((e) => ({ ...e, [name]: undefined }));
  }

  function validate(): boolean {
    const errs: Partial<Record<keyof FieldState, string>> = {};
    if (!fields.first_name.trim()) errs.first_name = 'First name is required';
    if (!fields.last_name.trim()) errs.last_name = 'Last name is required';
    setFieldErrors(errs);
    return Object.keys(errs).length === 0;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!validate()) return;

    setSubmitting(true);
    setError(null);

    try {
      if (isEdit && id) {
        const req: UpdateReaderRequest = {
          first_name: fields.first_name.trim(),
          last_name: fields.last_name.trim(),
          preferred_name: fields.preferred_name.trim() || undefined,
          notes: fields.notes.trim() || undefined,
          national_id: fields.national_id || undefined,
          contact_email: fields.contact_email || undefined,
          contact_phone: fields.contact_phone || undefined,
          date_of_birth: fields.date_of_birth || undefined,
        };
        await readersApi.update(id, req);
        navigate(`/readers/${id}`);
      } else {
        const req: CreateReaderRequest = {
          first_name: fields.first_name.trim(),
          last_name: fields.last_name.trim(),
          preferred_name: fields.preferred_name.trim() || undefined,
          notes: fields.notes.trim() || undefined,
          national_id: fields.national_id || undefined,
          contact_email: fields.contact_email || undefined,
          contact_phone: fields.contact_phone || undefined,
          date_of_birth: fields.date_of_birth || undefined,
        };
        const created = await readersApi.create(req);
        navigate(`/readers/${created.id}`);
      }
    } catch (err) {
      if (err instanceof HttpError && err.status === 422) {
        const field = err.body.field as keyof FieldState | undefined;
        if (field) {
          setFieldErrors((e) => ({ ...e, [field]: err.body.detail ?? 'Invalid value' }));
        } else {
          setError(err.body.detail ?? 'Validation error');
        }
      } else if (err instanceof HttpError && err.status === 409) {
        setError('Reader number already in use. Please choose a different one.');
      } else {
        setError(err instanceof Error ? err.message : 'Failed to save reader');
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (loadingInit) return <LoadingState />;

  return (
    <div style={{ maxWidth: '640px' }}>
      <button
        onClick={() => navigate(isEdit && id ? `/readers/${id}` : '/readers')}
        style={{ background: 'none', border: 'none', color: '#2563eb', cursor: 'pointer', fontSize: '0.8125rem', padding: '0 0 0.75rem 0' }}
      >
        ← {isEdit ? 'Back to reader' : 'Back to readers'}
      </button>

      <div style={{ fontSize: '1.125rem', fontWeight: 700, color: '#1a1a2e', marginBottom: '1.25rem' }}>
        {isEdit ? 'Edit Reader' : 'New Reader'}
      </div>

      {error && (
        <div role="alert" style={{ background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: '4px', padding: '0.625rem 0.875rem', fontSize: '0.8125rem', color: '#dc2626', marginBottom: '1rem' }}>
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '0.875rem' }}>
        {/* Basic section */}
        <div style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: '6px', overflow: 'hidden' }}>
          <div style={{ padding: '0.625rem 1rem', borderBottom: '1px solid #f3f4f6', fontWeight: 600, fontSize: '0.8125rem', color: '#374151', background: '#f9fafb' }}>
            Basic Information
          </div>
          <div style={{ padding: '1rem', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.875rem' }}>
            <div>
              <Field label="First name" name="first_name" value={fields.first_name} onChange={handleChange} required />
              {fieldErrors.first_name && <div style={{ fontSize: '0.75rem', color: '#dc2626', marginTop: '0.25rem' }}>{fieldErrors.first_name}</div>}
            </div>
            <div>
              <Field label="Last name" name="last_name" value={fields.last_name} onChange={handleChange} required />
              {fieldErrors.last_name && <div style={{ fontSize: '0.75rem', color: '#dc2626', marginTop: '0.25rem' }}>{fieldErrors.last_name}</div>}
            </div>
            <div style={{ gridColumn: '1 / -1' }}>
              <Field label="Preferred name" name="preferred_name" value={fields.preferred_name} onChange={handleChange} placeholder="Optional — shown in parentheses" />
            </div>
            <div style={{ gridColumn: '1 / -1' }}>
              <Field label="Notes" name="notes" value={fields.notes} onChange={handleChange} placeholder="Internal staff notes (not visible to reader)" />
            </div>
          </div>
        </div>

        {/* Sensitive section */}
        <div style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: '6px', overflow: 'hidden' }}>
          <div style={{ padding: '0.625rem 1rem', borderBottom: '1px solid #f3f4f6', fontWeight: 600, fontSize: '0.8125rem', color: '#374151', background: '#f9fafb' }}>
            Sensitive Information
          </div>
          <div style={{ padding: '0.625rem 1rem 0.375rem', background: '#fef3c7', borderBottom: '1px solid #fde68a', fontSize: '0.75rem', color: '#92400e' }}>
            These fields are encrypted at rest (AES-256-GCM). Leave blank to keep existing values when editing.
          </div>
          <div style={{ padding: '1rem', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.875rem' }}>
            <Field label="National ID" name="national_id" value={fields.national_id} onChange={handleChange} placeholder={isEdit ? '(leave blank to keep)' : 'Optional'} />
            <Field label="Date of Birth" name="date_of_birth" value={fields.date_of_birth} onChange={handleChange} type="date" />
            <Field label="Contact Email" name="contact_email" value={fields.contact_email} onChange={handleChange} type="email" placeholder={isEdit ? '(leave blank to keep)' : 'Optional'} />
            <Field label="Contact Phone" name="contact_phone" value={fields.contact_phone} onChange={handleChange} placeholder={isEdit ? '(leave blank to keep)' : 'Optional'} />
          </div>
        </div>

        {/* Actions */}
        <div style={{ display: 'flex', gap: '0.625rem', justifyContent: 'flex-end' }}>
          <button
            type="button"
            onClick={() => navigate(isEdit && id ? `/readers/${id}` : '/readers')}
            style={{ padding: '0.5rem 1rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: '4px', cursor: 'pointer', fontSize: '0.875rem' }}
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={submitting}
            style={{ padding: '0.5rem 1.25rem', background: '#2563eb', color: '#fff', border: 'none', borderRadius: '4px', cursor: submitting ? 'not-allowed' : 'pointer', fontSize: '0.875rem', fontWeight: 600, opacity: submitting ? 0.7 : 1 }}
          >
            {submitting ? 'Saving…' : isEdit ? 'Save changes' : 'Create reader'}
          </button>
        </div>
      </form>
    </div>
  );
}
