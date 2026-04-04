// FilterBar is a horizontal row of filter inputs for list/search views.
//
// Design: all filter state lives in the caller. FilterBar calls onChange on each
// input change and onApply/onClear on explicit user action. This keeps the
// component stateless and testable.
//
// Usage:
//   const [filters, setFilters] = useState({ status: '', search: '' });
//
//   <FilterBar
//     fields={[
//       { name: 'search', label: 'Search', type: 'text', placeholder: 'Name or reader #' },
//       { name: 'status', label: 'Status', type: 'select',
//         options: [{ value: 'active', label: 'Active' }, { value: 'frozen', label: 'Frozen' }] },
//     ]}
//     values={filters}
//     onChange={(name, value) => setFilters(f => ({ ...f, [name]: value }))}
//     onApply={() => refetch()}
//     onClear={() => { setFilters({ status: '', search: '' }); refetch(); }}
//   />

export interface FilterFieldDef {
  name: string;
  label: string;
  type: 'text' | 'select' | 'date';
  placeholder?: string;
  options?: { value: string; label: string }[];
}

interface FilterBarProps {
  fields: FilterFieldDef[];
  values: Record<string, string>;
  onChange: (name: string, value: string) => void;
  onApply: () => void;
  onClear: () => void;
  /** Whether a fetch is in progress (disables Apply button). */
  loading?: boolean;
}

const INPUT_STYLE: React.CSSProperties = {
  padding: '0.375rem 0.625rem',
  border: '1px solid #d1d5db',
  borderRadius: '4px',
  fontSize: '0.8125rem',
  background: '#fff',
  color: '#1a1a2e',
  minWidth: '140px',
};

export default function FilterBar({
  fields,
  values,
  onChange,
  onApply,
  onClear,
  loading = false,
}: FilterBarProps) {
  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') onApply();
  }

  return (
    <div
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'flex-end',
        gap: '0.75rem',
        padding: '0.875rem 1rem',
        background: '#f9fafb',
        border: '1px solid #e5e7eb',
        borderRadius: '6px',
        marginBottom: '1rem',
      }}
    >
      {fields.map((field) => (
        <div key={field.name} style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
          <label
            htmlFor={`filter-${field.name}`}
            style={{ fontSize: '0.6875rem', fontWeight: 600, color: '#6b7280', textTransform: 'uppercase', letterSpacing: '0.06em' }}
          >
            {field.label}
          </label>

          {field.type === 'select' ? (
            <select
              id={`filter-${field.name}`}
              value={values[field.name] ?? ''}
              onChange={(e) => onChange(field.name, e.target.value)}
              style={INPUT_STYLE}
            >
              <option value="">All</option>
              {field.options?.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          ) : (
            <input
              id={`filter-${field.name}`}
              type={field.type}
              value={values[field.name] ?? ''}
              placeholder={field.placeholder}
              onChange={(e) => onChange(field.name, e.target.value)}
              onKeyDown={handleKeyDown}
              style={INPUT_STYLE}
            />
          )}
        </div>
      ))}

      <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'flex-end' }}>
        <button
          onClick={onApply}
          disabled={loading}
          style={{
            padding: '0.375rem 0.875rem',
            background: '#2563eb',
            color: '#fff',
            border: 'none',
            borderRadius: '4px',
            cursor: loading ? 'not-allowed' : 'pointer',
            fontSize: '0.8125rem',
            fontWeight: 600,
            opacity: loading ? 0.7 : 1,
          }}
        >
          {loading ? 'Loading…' : 'Apply'}
        </button>
        <button
          onClick={onClear}
          style={{
            padding: '0.375rem 0.875rem',
            background: '#fff',
            color: '#374151',
            border: '1px solid #d1d5db',
            borderRadius: '4px',
            cursor: 'pointer',
            fontSize: '0.8125rem',
          }}
        >
          Clear
        </button>
      </div>
    </div>
  );
}
