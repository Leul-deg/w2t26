// ContentFormPage — create a new content item or view/edit an existing one.
// Also supports: submit for review, retract, publish, archive.

import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../../auth/AuthContext';
import { contentApi, GovernedContent, ContentType } from '../../api/content';

const INPUT_STYLE: React.CSSProperties = {
  padding: '0.375rem 0.625rem',
  border: '1px solid #d1d5db',
  borderRadius: 6,
  fontSize: '0.875rem',
  width: '100%',
  boxSizing: 'border-box',
};

const STATUS_ACTIONS: Record<string, { label: string; next: string; color: string }[]> = {
  draft:          [{ label: 'Submit for review', next: 'submit', color: '#059669' }],
  pending_review: [{ label: 'Retract to draft', next: 'retract', color: '#d97706' }],
  approved:       [{ label: 'Publish', next: 'publish', color: '#4f46e5' }],
  published:      [{ label: 'Archive', next: 'archive', color: '#6b7280' }],
  rejected:       [],
  archived:       [],
};

export default function ContentFormPage() {
  const { id } = useParams<{ id: string }>();
  const isNew = !id;
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canSubmit = hasPermission('content:submit');
  const canPublish = hasPermission('content:publish');

  const [item, setItem] = useState<GovernedContent | null>(null);
  const [title, setTitle] = useState('');
  const [contentType, setContentType] = useState<ContentType>('announcement');
  const [body, setBody] = useState('');
  const [loading, setLoading] = useState(!isNew);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionBusy, setActionBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const loadItem = useCallback(async () => {
    if (!id) return;
    try {
      const data = await contentApi.get(id);
      setItem(data);
      setTitle(data.title);
      setContentType(data.content_type);
      setBody(data.body ?? '');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { if (!isNew) loadItem(); }, [isNew, loadItem]);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) { setError('Title is required'); return; }
    setError(null);
    setSaving(true);
    try {
      if (isNew) {
        const created = await contentApi.create({ title: title.trim(), content_type: contentType, body: body.trim() || undefined });
        navigate(`/content/${created.id}`);
      } else if (id) {
        const updated = await contentApi.update(id, { title: title.trim(), body: body.trim() || undefined });
        setItem(updated);
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  };

  const handleAction = async (action: string) => {
    if (!id) return;
    setActionError(null);
    setActionBusy(true);
    try {
      let updated: GovernedContent;
      if (action === 'submit') updated = await contentApi.submit(id);
      else if (action === 'retract') updated = await contentApi.retract(id);
      else if (action === 'publish') updated = await contentApi.publish(id);
      else if (action === 'archive') updated = await contentApi.archive(id);
      else return;
      setItem(updated);
    } catch (err: unknown) {
      setActionError(err instanceof Error ? err.message : 'Action failed');
    } finally {
      setActionBusy(false);
    }
  };

  if (loading) return <p style={{ padding: '1.5rem', color: '#6b7280' }}>Loading…</p>;

  const isDraft = !item || item.status === 'draft';
  const canEdit = canSubmit && isDraft;
  const actions = item ? (STATUS_ACTIONS[item.status] ?? []) : [];
  const showPublishActions = actions.some(a => a.next === 'publish' || a.next === 'archive') && !canPublish ? false : true;

  return (
    <div style={{ maxWidth: 720, margin: '0 auto', padding: '1.5rem' }}>
      <div style={{ marginBottom: '1.5rem' }}>
        <button onClick={() => navigate('/content')} style={{ fontSize: '0.8125rem', color: '#6b7280', background: 'none', border: 'none', cursor: 'pointer', marginBottom: 8 }}>
          ← Content
        </button>
        <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
          <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>
            {isNew ? 'New content' : item?.title ?? 'Content'}
          </h1>
          {item && (
            <span style={{ fontSize: '0.75rem', padding: '0.125rem 0.5rem', borderRadius: '9999px', background: '#f3f4f6', color: '#374151' }}>
              {item.status.replace(/_/g, ' ')}
            </span>
          )}
        </div>
      </div>

      {/* Rejection notice */}
      {item?.status === 'rejected' && item.rejection_reason && (
        <div style={{ background: '#fef2f2', border: '1px solid #fecaca', borderRadius: 8, padding: '1rem', marginBottom: '1rem' }}>
          <p style={{ margin: 0, fontWeight: 600, color: '#991b1b', fontSize: '0.875rem' }}>Rejected</p>
          <p style={{ margin: '0.25rem 0 0', color: '#7f1d1d', fontSize: '0.875rem' }}>{item.rejection_reason}</p>
        </div>
      )}

      {/* Form */}
      <form onSubmit={handleSave}>
        {error && <p style={{ color: '#dc2626', marginBottom: '0.75rem', fontSize: '0.875rem' }}>{error}</p>}

        <div style={{ marginBottom: '1rem' }}>
          <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Title *</label>
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            style={INPUT_STYLE}
            disabled={!canEdit}
            required
          />
        </div>

        {isNew && (
          <div style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Content type *</label>
            <select value={contentType} onChange={(e) => setContentType(e.target.value as ContentType)} style={INPUT_STYLE}>
              <option value="announcement">Announcement</option>
              <option value="document">Document</option>
              <option value="digital_resource">Digital resource</option>
              <option value="policy">Policy</option>
            </select>
          </div>
        )}

        <div style={{ marginBottom: '1.25rem' }}>
          <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600, marginBottom: '0.25rem' }}>Body</label>
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            style={{ ...INPUT_STYLE, minHeight: 160, resize: 'vertical' }}
            disabled={!canEdit}
            placeholder="Content body…"
          />
        </div>

        {canEdit && (
          <div style={{ display: 'flex', gap: '0.75rem' }}>
            <button type="submit" disabled={saving}
              style={{ padding: '0.5rem 1.25rem', background: saving ? '#a5b4fc' : '#4f46e5', color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer', fontWeight: 600 }}>
              {saving ? 'Saving…' : (isNew ? 'Create draft' : 'Save')}
            </button>
            <button type="button" onClick={() => navigate('/content')}
              style={{ padding: '0.5rem 1rem', background: '#fff', color: '#374151', border: '1px solid #d1d5db', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer' }}>
              Cancel
            </button>
          </div>
        )}
      </form>

      {/* Lifecycle actions */}
      {item && showPublishActions && actions.length > 0 && (
        <div style={{ marginTop: '1.5rem', paddingTop: '1.5rem', borderTop: '1px solid #e5e7eb' }}>
          {actionError && <p style={{ color: '#dc2626', marginBottom: '0.5rem', fontSize: '0.875rem' }}>{actionError}</p>}
          <div style={{ display: 'flex', gap: '0.75rem' }}>
            {actions.map((a) => (
              (a.next === 'publish' || a.next === 'archive') && !canPublish ? null : (
                <button
                  key={a.next}
                  onClick={() => handleAction(a.next)}
                  disabled={actionBusy}
                  style={{ padding: '0.5rem 1rem', background: a.color, color: '#fff', border: 'none', borderRadius: 6, fontSize: '0.875rem', cursor: 'pointer', fontWeight: 600 }}
                >
                  {a.label}
                </button>
              )
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
