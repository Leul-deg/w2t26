import { useCallback, useEffect, useMemo, useState } from 'react';
import { useAuth } from '../../auth/AuthContext';
import { reportsApi, ReportAggregate, ReportDefinition, ReportRunResult } from '../../api/reports';

const INPUT: React.CSSProperties = {
  padding: '0.375rem 0.625rem',
  border: '1px solid #d1d5db',
  borderRadius: 6,
  fontSize: '0.875rem',
  background: '#fff',
};

const KPI_ALIAS_TEXT = [
  'occupancy_rate -> slot_utilization_rate',
  'revpar -> resource_yield_per_available_slot',
  'revenue_mix -> enrollment_mix_by_category',
  'room_type -> venue_type',
  'channel -> enrollment_channel',
].join(' | ');

function todayMinus(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() - days);
  return d.toISOString().slice(0, 10);
}

function downloadBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

function pretty(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export default function ReportsPage() {
  const { hasPermission, hasRole } = useAuth();
  const canRead = hasPermission('reports:read');
  const canExport = hasPermission('reports:export');
  const canAdmin = hasPermission('reports:admin');
  const isAdmin = hasRole('administrator');

  const [definitions, setDefinitions] = useState<ReportDefinition[]>([]);
  const [selectedDefinitionId, setSelectedDefinitionId] = useState('');
  const [from, setFrom] = useState(todayMinus(30));
  const [to, setTo] = useState(todayMinus(0));
  const [filterText, setFilterText] = useState('');
  const [branchId, setBranchId] = useState('');

  const [runResult, setRunResult] = useState<ReportRunResult | null>(null);
  const [aggregates, setAggregates] = useState<ReportAggregate[]>([]);

  const [loadingDefs, setLoadingDefs] = useState(true);
  const [running, setRunning] = useState(false);
  const [loadingAggs, setLoadingAggs] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [recalculating, setRecalculating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const filters = useMemo(() => {
    const out: Record<string, string> = {};
    if (filterText.trim()) out.category = filterText.trim();
    return out;
  }, [filterText]);

  const selectedDefinition = useMemo(
    () => definitions.find((d) => d.id === selectedDefinitionId) ?? null,
    [definitions, selectedDefinitionId],
  );

  const loadDefinitions = useCallback(async () => {
    if (!canRead) return;
    setLoadingDefs(true);
    setError(null);
    try {
      const defs = await reportsApi.listDefinitions();
      setDefinitions(defs);
      setSelectedDefinitionId(prev => prev || (defs.length > 0 ? defs[0].id : ''));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load report definitions');
    } finally {
      setLoadingDefs(false);
    }
  }, [canRead]);

  const loadAggregates = useCallback(async () => {
    if (!canRead || !selectedDefinitionId) return;
    setLoadingAggs(true);
    try {
      const rows = await reportsApi.listAggregates({
        definition_id: selectedDefinitionId,
        from,
        to,
        branch_id: isAdmin ? branchId.trim() || undefined : undefined,
      });
      setAggregates(rows);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load cached aggregates');
    } finally {
      setLoadingAggs(false);
    }
  }, [canRead, selectedDefinitionId, from, to, isAdmin, branchId]);

  useEffect(() => {
    loadDefinitions();
  }, [loadDefinitions]);

  useEffect(() => {
    if (selectedDefinitionId) {
      loadAggregates();
    }
  }, [selectedDefinitionId, from, to, loadAggregates]);

  async function handleRun() {
    if (!selectedDefinitionId) return;
    if (isAdmin && !branchId.trim()) {
      setError('Branch ID is required for administrator report runs and exports.');
      setNotice(null);
      return;
    }
    setRunning(true);
    setError(null);
    setNotice(null);
    try {
      const result = await reportsApi.run({
        definition_id: selectedDefinitionId,
        from,
        to,
        branch_id: isAdmin ? branchId.trim() : undefined,
        filters,
      });
      setRunResult(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to run report');
    } finally {
      setRunning(false);
    }
  }

  async function handleExport() {
    if (!selectedDefinitionId) return;
    if (isAdmin && !branchId.trim()) {
      setError('Branch ID is required for administrator report runs and exports.');
      setNotice(null);
      return;
    }
    setExporting(true);
    setError(null);
    setNotice(null);
    try {
      const { blob, fileName, exportJobId } = await reportsApi.export({
        definition_id: selectedDefinitionId,
        from,
        to,
        branch_id: isAdmin ? branchId.trim() : undefined,
        filters,
      });
      downloadBlob(blob, fileName);
      setNotice(exportJobId ? `Report exported. Audit job: ${exportJobId}` : 'Report exported.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to export report');
    } finally {
      setExporting(false);
    }
  }

  async function handleRecalculate() {
    if (!selectedDefinitionId) return;
    setRecalculating(true);
    setError(null);
    setNotice(null);
    try {
      const result = await reportsApi.recalculate({
        definition_id: selectedDefinitionId,
        from,
        to,
      });
      setNotice(`Recalculated ${result.aggregates_computed} aggregate row(s).`);
      await loadAggregates();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to recalculate aggregates');
    } finally {
      setRecalculating(false);
    }
  }

  if (!canRead) {
    return <p style={{ padding: '1.5rem', color: '#dc2626' }}>You do not have permission to view reports.</p>;
  }

  const tableColumns = runResult?.rows.length ? Object.keys(runResult.rows[0]) : [];

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto', padding: '1.5rem' }}>
      <div style={{ marginBottom: '1.5rem' }}>
        <h1 style={{ fontSize: '1.25rem', fontWeight: 700, margin: 0 }}>Reports & Analytics</h1>
        <p style={{ color: '#6b7280', fontSize: '0.875rem', marginTop: '0.5rem' }}>
          Offline-friendly reports with nightly pre-aggregation, on-demand recalculation, and audited CSV exports.
        </p>
      </div>

      <div style={{ background: '#eff6ff', border: '1px solid #bfdbfe', borderRadius: 8, padding: '1rem', marginBottom: '1rem' }}>
        <p style={{ margin: 0, fontWeight: 600, color: '#1d4ed8', fontSize: '0.875rem' }}>
          KPI ambiguity handling
        </p>
        <p style={{ margin: '0.375rem 0 0', color: '#1e40af', fontSize: '0.8125rem' }}>
          Hospitality-style terms are preserved as aliases over LMS-native metrics: {KPI_ALIAS_TEXT}
        </p>
      </div>

      <section style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1rem', marginBottom: '1rem' }}>
        <div style={{ display: 'grid', gridTemplateColumns: isAdmin ? '2fr 1fr 1fr 1fr 1.5fr' : '2fr 1fr 1fr 1fr', gap: '0.75rem', alignItems: 'end' }}>
          <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600 }}>
            Report definition
            <select
              value={selectedDefinitionId}
              onChange={(e) => setSelectedDefinitionId(e.target.value)}
              style={{ ...INPUT, width: '100%', marginTop: '0.25rem' }}
              disabled={loadingDefs}
            >
              {definitions.map((d) => (
                <option key={d.id} value={d.id}>{d.name}</option>
              ))}
            </select>
          </label>

          <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600 }}>
            From
            <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} style={{ ...INPUT, width: '100%', marginTop: '0.25rem' }} />
          </label>

          <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600 }}>
            To
            <input type="date" value={to} onChange={(e) => setTo(e.target.value)} style={{ ...INPUT, width: '100%', marginTop: '0.25rem' }} />
          </label>

          <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600 }}>
            Optional category filter
            <input value={filterText} onChange={(e) => setFilterText(e.target.value)} style={{ ...INPUT, width: '100%', marginTop: '0.25rem' }} placeholder="category" />
          </label>

          {isAdmin && (
            <label style={{ display: 'block', fontSize: '0.8125rem', fontWeight: 600 }}>
              Branch ID {canAdmin || canExport ? '*' : '(optional)'}
              <input
                value={branchId}
                onChange={(e) => setBranchId(e.target.value)}
                style={{ ...INPUT, width: '100%', marginTop: '0.25rem', fontFamily: 'monospace' }}
                placeholder="Branch UUID for admin-scoped reports"
              />
            </label>
          )}
        </div>

        {selectedDefinition?.description && (
          <p style={{ margin: '0.75rem 0 0', color: '#6b7280', fontSize: '0.8125rem' }}>{selectedDefinition.description}</p>
        )}

        {selectedDefinition?.metric_aliases && (
          <pre style={{ marginTop: '0.75rem', padding: '0.75rem', background: '#f9fafb', borderRadius: 6, fontSize: '0.75rem', overflowX: 'auto' }}>
            {pretty(selectedDefinition.metric_aliases)}
          </pre>
        )}

        <div style={{ display: 'flex', gap: '0.75rem', marginTop: '1rem', flexWrap: 'wrap' }}>
          <button type="button" onClick={handleRun} disabled={running || loadingDefs || !selectedDefinitionId} style={buttonStyle('#4f46e5', running)}>
            {running ? 'Running…' : 'Run live report'}
          </button>
          {canExport && (
            <button type="button" onClick={handleExport} disabled={exporting || !selectedDefinitionId} style={buttonStyle('#059669', exporting)}>
              {exporting ? 'Exporting…' : 'Export CSV (Excel-compatible)'}
            </button>
          )}
          {canAdmin && (
            <button type="button" onClick={handleRecalculate} disabled={recalculating || !selectedDefinitionId} style={buttonStyle('#d97706', recalculating)}>
              {recalculating ? 'Recalculating…' : 'Recalculate cached aggregates'}
            </button>
          )}
          <button type="button" onClick={loadAggregates} disabled={loadingAggs || !selectedDefinitionId} style={buttonStyle('#374151', loadingAggs)}>
            {loadingAggs ? 'Refreshing…' : 'Refresh cached aggregates'}
          </button>
        </div>
      </section>

      {error && <p style={{ color: '#dc2626', fontSize: '0.875rem' }}>{error}</p>}
      {notice && <p style={{ color: '#065f46', fontSize: '0.875rem' }}>{notice}</p>}

      <section style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1rem', marginBottom: '1rem' }}>
        <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginTop: 0 }}>Live report results</h2>
        {!runResult && <p style={{ color: '#6b7280', fontSize: '0.875rem' }}>Run a report to see live results.</p>}
        {runResult && runResult.rows.length === 0 && <p style={{ color: '#6b7280', fontSize: '0.875rem' }}>No rows returned for the selected range.</p>}
        {runResult && runResult.rows.length > 0 && (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
              <thead>
                <tr style={{ background: '#f3f4f6', borderBottom: '1px solid #e5e7eb' }}>
                  {tableColumns.map((col) => (
                    <th key={col} style={{ textAlign: 'left', padding: '0.5rem 0.75rem', fontWeight: 600 }}>{col}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {runResult.rows.map((row, index) => (
                  <tr key={index} style={{ borderBottom: '1px solid #f3f4f6' }}>
                    {tableColumns.map((col) => (
                      <td key={col} style={{ padding: '0.5rem 0.75rem', verticalAlign: 'top' }}>{String(row[col] ?? '—')}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section style={{ background: '#fff', border: '1px solid #e5e7eb', borderRadius: 8, padding: '1rem' }}>
        <h2 style={{ fontSize: '0.9375rem', fontWeight: 600, marginTop: 0 }}>Cached aggregates</h2>
        {loadingAggs && <p style={{ color: '#6b7280', fontSize: '0.875rem' }}>Loading cached aggregates…</p>}
        {!loadingAggs && aggregates.length === 0 && <p style={{ color: '#6b7280', fontSize: '0.875rem' }}>No cached aggregates found for the selected range.</p>}
        {!loadingAggs && aggregates.length > 0 && (
          <div style={{ display: 'grid', gap: '0.75rem' }}>
            {aggregates.map((agg) => (
              <div key={agg.id} style={{ border: '1px solid #e5e7eb', borderRadius: 8, padding: '0.875rem' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', gap: '1rem', flexWrap: 'wrap', marginBottom: '0.5rem' }}>
                  <strong style={{ fontSize: '0.875rem' }}>{agg.period_start} → {agg.period_end}</strong>
                  <span style={{ color: '#6b7280', fontSize: '0.75rem' }}>Generated {new Date(agg.generated_at).toLocaleString()}</span>
                </div>
                <pre style={{ margin: 0, fontSize: '0.75rem', overflowX: 'auto', background: '#f9fafb', padding: '0.75rem', borderRadius: 6 }}>
                  {pretty(agg.aggregate_data)}
                </pre>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function buttonStyle(background: string, loading: boolean): React.CSSProperties {
  return {
    padding: '0.5rem 1rem',
    background,
    color: '#fff',
    border: 'none',
    borderRadius: 6,
    fontSize: '0.875rem',
    cursor: loading ? 'not-allowed' : 'pointer',
    opacity: loading ? 0.7 : 1,
    fontWeight: 600,
  };
}
