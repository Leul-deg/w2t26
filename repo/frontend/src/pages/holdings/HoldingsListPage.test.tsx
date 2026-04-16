import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import HoldingsListPage from './HoldingsListPage';

const { mockUseAuth, mockHoldingsApi } = vi.hoisted(() => ({
  mockUseAuth: vi.fn(),
  mockHoldingsApi: {
    list: vi.fn(),
    get: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    deactivate: vi.fn(),
    getCopies: vi.fn(),
    addCopy: vi.fn(),
    getStatuses: vi.fn(),
    getCopy: vi.fn(),
    updateCopy: vi.fn(),
    updateCopyStatus: vi.fn(),
    lookupByBarcode: vi.fn(),
  },
}));

vi.mock('../../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('../../api/holdings', () => ({
  holdingsApi: mockHoldingsApi,
}));

const emptyPage = { items: [], total: 0, page: 1, per_page: 20, total_pages: 0 };

const sampleHolding = {
  id: 'holding-1',
  branch_id: 'branch-1',
  title: 'The Go Programming Language',
  author: 'Alan Donovan',
  language: 'en',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

function renderPage() {
  return render(
    <MemoryRouter>
      <HoldingsListPage />
    </MemoryRouter>,
  );
}

describe('HoldingsListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({ hasPermission: () => false });
    mockHoldingsApi.list.mockResolvedValue(emptyPage);
  });

  it('renders the Holdings heading', async () => {
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/Holdings/i)).toBeInTheDocument();
    });
  });

  it('shows holding rows when API returns data', async () => {
    mockHoldingsApi.list.mockResolvedValue({
      items: [sampleHolding],
      total: 1,
      page: 1,
      per_page: 20,
      total_pages: 1,
    });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('The Go Programming Language')).toBeInTheDocument();
    });
  });

  it('shows "No holdings found" when list is empty', async () => {
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No holdings found/i)).toBeInTheDocument();
    });
  });

  it('hides the "+ Add Holding" button for read-only users', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p !== 'holdings:write' });
    renderPage();
    await waitFor(() => {
      expect(screen.queryByText(/\+ Add Holding/i)).not.toBeInTheDocument();
    });
  });

  it('shows the "+ Add Holding" button for users with holdings:write', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === 'holdings:write' });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/\+ Add Holding/i)).toBeInTheDocument();
    });
  });

  it('calls holdingsApi.list on mount', async () => {
    renderPage();
    await waitFor(() => {
      expect(mockHoldingsApi.list).toHaveBeenCalled();
    });
  });
});
