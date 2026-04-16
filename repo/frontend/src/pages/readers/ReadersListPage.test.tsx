import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import ReadersListPage from './ReadersListPage';

const { mockUseAuth, mockReadersApi } = vi.hoisted(() => ({
  mockUseAuth: vi.fn(),
  mockReadersApi: {
    list: vi.fn(),
    listStatuses: vi.fn(),
    get: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    updateStatus: vi.fn(),
    reveal: vi.fn(),
    getLoanHistory: vi.fn(),
    getCurrentHoldings: vi.fn(),
  },
}));

vi.mock('../../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('../../api/readers', () => ({
  readersApi: mockReadersApi,
}));

const emptyPage = { items: [], total: 0, page: 1, per_page: 20, total_pages: 0 };

const sampleReader = {
  id: 'reader-1',
  branch_id: 'branch-1',
  reader_number: 'RDR-001',
  status_code: 'active',
  first_name: 'Alice',
  last_name: 'Smith',
  registered_at: '2026-01-01T00:00:00Z',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  sensitive_fields: {},
};

function renderPage() {
  return render(
    <MemoryRouter>
      <ReadersListPage />
    </MemoryRouter>,
  );
}

describe('ReadersListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({ hasPermission: () => false });
    mockReadersApi.list.mockResolvedValue(emptyPage);
    mockReadersApi.listStatuses.mockResolvedValue([]);
  });

  it('renders the Readers heading', async () => {
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Readers')).toBeInTheDocument();
    });
  });

  it('shows reader rows when API returns data', async () => {
    mockReadersApi.list.mockResolvedValue({
      items: [sampleReader],
      total: 1,
      page: 1,
      per_page: 20,
      total_pages: 1,
    });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('RDR-001')).toBeInTheDocument();
    });
    expect(screen.getByText(/Smith, Alice/i)).toBeInTheDocument();
  });

  it('shows "No readers found" when the list is empty', async () => {
    mockReadersApi.list.mockResolvedValue(emptyPage);
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No readers found/i)).toBeInTheDocument();
    });
  });

  it('hides the "+ New Reader" button for read-only users', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p !== 'readers:write' });
    renderPage();
    await waitFor(() => {
      expect(screen.queryByText(/\+ New Reader/i)).not.toBeInTheDocument();
    });
  });

  it('shows the "+ New Reader" button for users with readers:write', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === 'readers:write' });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/\+ New Reader/i)).toBeInTheDocument();
    });
  });

  it('calls list with search param when filter is applied', async () => {
    mockReadersApi.list.mockResolvedValue(emptyPage);
    renderPage();

    await waitFor(() => {
      expect(mockReadersApi.list).toHaveBeenCalled();
    });

    const searchInput = screen.getByPlaceholderText(/Name or reader #/i);
    fireEvent.change(searchInput, { target: { value: 'Alice' } });

    const applyButton = screen.getByRole('button', { name: /apply/i });
    fireEvent.click(applyButton);

    await waitFor(() => {
      expect(mockReadersApi.list).toHaveBeenCalledWith(
        expect.objectContaining({ search: 'Alice' }),
      );
    });
  });
});
