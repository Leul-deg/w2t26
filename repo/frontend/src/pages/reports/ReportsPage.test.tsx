import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import ReportsPage from './ReportsPage';

const { mockUseAuth, mockReportsApi } = vi.hoisted(() => ({
  mockUseAuth: vi.fn(),
  mockReportsApi: {
    listDefinitions: vi.fn(),
    run: vi.fn(),
    listAggregates: vi.fn(),
    recalculate: vi.fn(),
    export: vi.fn(),
  },
}));

vi.mock('../../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('../../api/reports', () => ({
  reportsApi: mockReportsApi,
}));

const sampleDefinition = {
  id: 'def-1',
  name: 'Circulation Overview',
  description: 'Overview of circulation activity',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

const sampleRunResult = {
  definition: sampleDefinition,
  from: '2026-04-01',
  to: '2026-04-15',
  rows: [{ branch: 'Main', checkouts: 42 }],
  row_count: 1,
};

function renderPage() {
  return render(
    <MemoryRouter>
      <ReportsPage />
    </MemoryRouter>,
  );
}

describe('ReportsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      hasPermission: () => false,
      hasRole: () => false,
    });
    mockReportsApi.listDefinitions.mockResolvedValue([]);
    mockReportsApi.listAggregates.mockResolvedValue([]);
    mockReportsApi.run.mockResolvedValue(sampleRunResult);
    mockReportsApi.recalculate.mockResolvedValue({ aggregates_computed: 3 });
    mockReportsApi.export.mockResolvedValue({
      blob: new Blob(['csv data'], { type: 'text/csv' }),
      fileName: 'report.csv',
    });
  });

  it('shows permission error for users without reports:read', () => {
    mockUseAuth.mockReturnValue({ hasPermission: () => false, hasRole: () => false });
    renderPage();
    expect(screen.getByText(/do not have permission/i)).toBeInTheDocument();
  });

  it('renders Reports & Analytics heading for authorized users', async () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'reports:read',
      hasRole: () => false,
    });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/Reports & Analytics/i)).toBeInTheDocument();
    });
  });

  it('loads and shows report definitions in dropdown', async () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'reports:read',
      hasRole: () => false,
    });
    mockReportsApi.listDefinitions.mockResolvedValue([sampleDefinition]);

    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Circulation Overview')).toBeInTheDocument();
    });
  });

  it('calls reportsApi.run when "Run live report" is clicked', async () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'reports:read',
      hasRole: () => false,
    });
    mockReportsApi.listDefinitions.mockResolvedValue([sampleDefinition]);

    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Circulation Overview')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Run live report/i }));

    await waitFor(() => {
      expect(mockReportsApi.run).toHaveBeenCalled();
    });
  });

  it('shows run result rows after running a report', async () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'reports:read',
      hasRole: () => false,
    });
    mockReportsApi.listDefinitions.mockResolvedValue([sampleDefinition]);

    renderPage();
    await waitFor(() => expect(screen.getByText('Circulation Overview')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /Run live report/i }));

    await waitFor(() => {
      // Row value "42" from sampleRunResult.rows[0].checkouts should appear.
      expect(screen.getByText('42')).toBeInTheDocument();
    });
  });

  it('hides Export CSV button for users without reports:export', async () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'reports:read',
      hasRole: () => false,
    });
    mockReportsApi.listDefinitions.mockResolvedValue([sampleDefinition]);

    renderPage();
    await waitFor(() => expect(screen.getByText('Circulation Overview')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /Export CSV/i })).not.toBeInTheDocument();
  });

  it('shows Export CSV button for users with reports:export', async () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'reports:read' || p === 'reports:export',
      hasRole: () => false,
    });
    mockReportsApi.listDefinitions.mockResolvedValue([sampleDefinition]);

    renderPage();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Export CSV/i })).toBeInTheDocument();
    });
  });

  it('hides Recalculate button for users without reports:admin', async () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'reports:read',
      hasRole: () => false,
    });
    mockReportsApi.listDefinitions.mockResolvedValue([sampleDefinition]);

    renderPage();
    await waitFor(() => expect(screen.getByText('Circulation Overview')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /Recalculate/i })).not.toBeInTheDocument();
  });
});
