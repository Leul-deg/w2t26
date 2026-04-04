import { useEffect, useState } from 'react';
import { usersApi, UserListItem, CreateUserRequest } from '../../api/users';
import { useAuth } from '../../auth/AuthContext';

export default function UsersPage() {
  const { user } = useAuth();
  const canWrite = user?.permissions?.includes('users:write') ?? false;
  const canAdmin = user?.permissions?.includes('users:admin') ?? false;

  const [items, setItems] = useState<UserListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState<CreateUserRequest>({ username: '', email: '', password: '' });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const [editID, setEditID] = useState<string | null>(null);
  const [editEmail, setEditEmail] = useState('');
  const [editActive, setEditActive] = useState(true);
  const [saving, setSaving] = useState(false);

  async function load(p: number) {
    setLoading(true);
    setError(null);
    try {
      const res = await usersApi.list({ page: p, per_page: 20 });
      setItems(res.items ?? []);
      setTotal(res.total);
      setTotalPages(res.total_pages);
      setPage(p);
    } catch (e: any) {
      setError(e?.message ?? 'Failed to load users');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(1); }, []);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!createForm.username || !createForm.email || !createForm.password) return;
    setCreating(true);
    setCreateError(null);
    try {
      await usersApi.create(createForm);
      setShowCreate(false);
      setCreateForm({ username: '', email: '', password: '' });
      load(page);
    } catch (e: any) {
      setCreateError(e?.message ?? 'Failed to create user');
    } finally {
      setCreating(false);
    }
  }

  function startEdit(u: UserListItem) {
    setEditID(u.id);
    setEditEmail(u.email);
    setEditActive(u.is_active);
  }

  async function handleSave(id: string) {
    setSaving(true);
    try {
      await usersApi.update(id, { email: editEmail, is_active: editActive });
      setEditID(null);
      load(page);
    } catch {
      // leave edit mode open on error
    } finally {
      setSaving(false);
    }
  }

  async function toggleActive(u: UserListItem) {
    try {
      await usersApi.update(u.id, { is_active: !u.is_active });
      load(page);
    } catch {
      // ignore
    }
  }

  const colStyle: React.CSSProperties = {
    padding: '0.5rem 0.75rem',
    textAlign: 'left',
    borderBottom: '1px solid #e5e7eb',
    fontSize: '0.8125rem',
    whiteSpace: 'nowrap',
  };
  const thStyle: React.CSSProperties = {
    ...colStyle,
    background: '#f9fafb',
    fontWeight: 600,
    color: '#374151',
  };

  return (
    <div>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: '1rem',
        }}
      >
        <div style={{ fontSize: '1.125rem', fontWeight: 700, color: '#1a1a2e' }}>
          User Management
        </div>
        {canWrite && (
          <button
            onClick={() => setShowCreate(true)}
            style={{
              background: '#1a1a2e',
              color: '#fff',
              border: 'none',
              borderRadius: '4px',
              padding: '0.5rem 1rem',
              fontSize: '0.8125rem',
              cursor: 'pointer',
            }}
          >
            New User
          </button>
        )}
      </div>

      {showCreate && (
        <div
          style={{
            background: '#f9fafb',
            border: '1px solid #e5e7eb',
            borderRadius: '4px',
            padding: '1rem',
            marginBottom: '1rem',
            maxWidth: '480px',
          }}
        >
          <div style={{ fontWeight: 600, marginBottom: '0.75rem', fontSize: '0.875rem' }}>
            Create User
          </div>
          <form onSubmit={handleCreate}>
            {(['username', 'email', 'password'] as const).map((field) => (
              <div key={field} style={{ marginBottom: '0.5rem' }}>
                <label style={{ display: 'block', fontSize: '0.75rem', color: '#6b7280', marginBottom: '0.25rem' }}>
                  {field.charAt(0).toUpperCase() + field.slice(1)}
                </label>
                <input
                  type={field === 'password' ? 'password' : 'text'}
                  value={createForm[field]}
                  onChange={(e) => setCreateForm((f) => ({ ...f, [field]: e.target.value }))}
                  style={{
                    width: '100%',
                    padding: '0.375rem 0.5rem',
                    border: '1px solid #d1d5db',
                    borderRadius: '4px',
                    fontSize: '0.8125rem',
                    boxSizing: 'border-box',
                  }}
                />
              </div>
            ))}
            {createError && (
              <div style={{ color: '#b91c1c', fontSize: '0.75rem', marginBottom: '0.5rem' }}>{createError}</div>
            )}
            <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.75rem' }}>
              <button
                type="submit"
                disabled={creating}
                style={{
                  background: '#1a1a2e',
                  color: '#fff',
                  border: 'none',
                  borderRadius: '4px',
                  padding: '0.375rem 0.75rem',
                  fontSize: '0.8125rem',
                  cursor: creating ? 'default' : 'pointer',
                }}
              >
                {creating ? 'Creating…' : 'Create'}
              </button>
              <button
                type="button"
                onClick={() => { setShowCreate(false); setCreateError(null); }}
                style={{
                  background: 'transparent',
                  border: '1px solid #d1d5db',
                  borderRadius: '4px',
                  padding: '0.375rem 0.75rem',
                  fontSize: '0.8125rem',
                  cursor: 'pointer',
                }}
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {loading && <div style={{ color: '#6b7280', fontSize: '0.875rem' }}>Loading…</div>}
      {error && <div style={{ color: '#b91c1c', fontSize: '0.875rem' }}>{error}</div>}

      {!loading && !error && (
        <>
          <div style={{ fontSize: '0.75rem', color: '#6b7280', marginBottom: '0.5rem' }}>
            {total} user{total !== 1 ? 's' : ''}
          </div>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr>
                  <th style={thStyle}>Username</th>
                  <th style={thStyle}>Email</th>
                  <th style={thStyle}>Roles</th>
                  <th style={thStyle}>Branches</th>
                  <th style={thStyle}>Status</th>
                  {(canWrite || canAdmin) && <th style={thStyle}>Actions</th>}
                </tr>
              </thead>
              <tbody>
                {items.map((u) => (
                  <tr key={u.id} style={{ background: '#fff' }}>
                    <td style={colStyle}>{u.username}</td>
                    <td style={colStyle}>
                      {editID === u.id ? (
                        <input
                          value={editEmail}
                          onChange={(e) => setEditEmail(e.target.value)}
                          style={{
                            padding: '0.25rem 0.375rem',
                            border: '1px solid #d1d5db',
                            borderRadius: '4px',
                            fontSize: '0.8125rem',
                            width: '200px',
                          }}
                        />
                      ) : (
                        u.email
                      )}
                    </td>
                    <td style={colStyle}>
                      {(u.roles ?? []).map((r) => r.name).join(', ') || '—'}
                    </td>
                    <td style={colStyle}>
                      {(u.branch_ids ?? []).length > 0 ? u.branch_ids.join(', ') : '—'}
                    </td>
                    <td style={colStyle}>
                      {editID === u.id ? (
                        <label style={{ display: 'flex', alignItems: 'center', gap: '0.25rem', cursor: 'pointer' }}>
                          <input
                            type="checkbox"
                            checked={editActive}
                            onChange={(e) => setEditActive(e.target.checked)}
                          />
                          Active
                        </label>
                      ) : (
                        <span
                          style={{
                            padding: '0.125rem 0.5rem',
                            borderRadius: '9999px',
                            fontSize: '0.75rem',
                            background: u.is_active ? '#dcfce7' : '#fee2e2',
                            color: u.is_active ? '#166534' : '#991b1b',
                          }}
                        >
                          {u.is_active ? 'Active' : 'Inactive'}
                        </span>
                      )}
                    </td>
                    {(canWrite || canAdmin) && (
                      <td style={colStyle}>
                        {editID === u.id ? (
                          <div style={{ display: 'flex', gap: '0.375rem' }}>
                            <button
                              onClick={() => handleSave(u.id)}
                              disabled={saving}
                              style={{
                                fontSize: '0.75rem',
                                padding: '0.25rem 0.5rem',
                                background: '#1a1a2e',
                                color: '#fff',
                                border: 'none',
                                borderRadius: '3px',
                                cursor: saving ? 'default' : 'pointer',
                              }}
                            >
                              Save
                            </button>
                            <button
                              onClick={() => setEditID(null)}
                              style={{
                                fontSize: '0.75rem',
                                padding: '0.25rem 0.5rem',
                                background: 'transparent',
                                border: '1px solid #d1d5db',
                                borderRadius: '3px',
                                cursor: 'pointer',
                              }}
                            >
                              Cancel
                            </button>
                          </div>
                        ) : (
                          <div style={{ display: 'flex', gap: '0.375rem' }}>
                            {canWrite && (
                              <button
                                onClick={() => startEdit(u)}
                                style={{
                                  fontSize: '0.75rem',
                                  padding: '0.25rem 0.5rem',
                                  background: 'transparent',
                                  border: '1px solid #d1d5db',
                                  borderRadius: '3px',
                                  cursor: 'pointer',
                                }}
                              >
                                Edit
                              </button>
                            )}
                            {canWrite && (
                              <button
                                onClick={() => toggleActive(u)}
                                style={{
                                  fontSize: '0.75rem',
                                  padding: '0.25rem 0.5rem',
                                  background: 'transparent',
                                  border: '1px solid #d1d5db',
                                  borderRadius: '3px',
                                  cursor: 'pointer',
                                }}
                              >
                                {u.is_active ? 'Deactivate' : 'Activate'}
                              </button>
                            )}
                          </div>
                        )}
                      </td>
                    )}
                  </tr>
                ))}
                {items.length === 0 && (
                  <tr>
                    <td
                      colSpan={canWrite || canAdmin ? 6 : 5}
                      style={{ ...colStyle, textAlign: 'center', color: '#6b7280', padding: '2rem' }}
                    >
                      No users found.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div style={{ display: 'flex', gap: '0.5rem', marginTop: '0.75rem', alignItems: 'center' }}>
              <button
                disabled={page <= 1}
                onClick={() => load(page - 1)}
                style={{
                  padding: '0.25rem 0.625rem',
                  fontSize: '0.8125rem',
                  border: '1px solid #d1d5db',
                  borderRadius: '3px',
                  cursor: page <= 1 ? 'default' : 'pointer',
                  opacity: page <= 1 ? 0.4 : 1,
                  background: 'transparent',
                }}
              >
                ‹
              </button>
              <span style={{ fontSize: '0.8125rem', color: '#6b7280' }}>
                Page {page} of {totalPages}
              </span>
              <button
                disabled={page >= totalPages}
                onClick={() => load(page + 1)}
                style={{
                  padding: '0.25rem 0.625rem',
                  fontSize: '0.8125rem',
                  border: '1px solid #d1d5db',
                  borderRadius: '3px',
                  cursor: page >= totalPages ? 'default' : 'pointer',
                  opacity: page >= totalPages ? 0.4 : 1,
                  background: 'transparent',
                }}
              >
                ›
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
