import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import ModerationQueuePage from './ModerationQueuePage';

const { mockUseAuth, mockModerationApi } = vi.hoisted(() => ({
  mockUseAuth: vi.fn(),
  mockModerationApi: {
    listQueue: vi.fn(),
    getItem: vi.fn(),
    assignItem: vi.fn(),
    decideItem: vi.fn(),
  },
}));

vi.mock('../../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('../../api/moderation', () => ({
  moderationApi: mockModerationApi,
}));

const emptyPage = { items: [], total: 0, page: 1, per_page: 20, total_pages: 0 };

const sampleItem = {
  id: 'mod-item-1',
  content_id: 'abcd1234-5678-0000-0000-000000000001',
  status: 'pending' as const,
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-01T00:00:00Z',
};

function renderPage() {
  return render(
    <MemoryRouter>
      <ModerationQueuePage />
    </MemoryRouter>,
  );
}

describe('ModerationQueuePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({ hasPermission: () => false });
    mockModerationApi.listQueue.mockResolvedValue(emptyPage);
  });

  it('shows permission error for users without content:moderate', () => {
    mockUseAuth.mockReturnValue({ hasPermission: () => false });
    renderPage();
    expect(screen.getByText(/do not have permission/i)).toBeInTheDocument();
  });

  it('renders the Moderation Queue heading for authorized users', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === 'content:moderate' });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText('Moderation Queue')).toBeInTheDocument();
    });
  });

  it('shows "No items in queue" when queue is empty', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === 'content:moderate' });
    renderPage();
    await waitFor(() => {
      expect(screen.getByText(/No items in queue/i)).toBeInTheDocument();
    });
  });

  it('shows moderation items when queue has entries', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === 'content:moderate' });
    mockModerationApi.listQueue.mockResolvedValue({
      items: [sampleItem],
      total: 1,
      page: 1,
      per_page: 20,
      total_pages: 1,
    });
    renderPage();
    await waitFor(() => {
      // Page renders content_id.slice(0, 8) + '…'
      expect(screen.getByText(/abcd1234/i)).toBeInTheDocument();
    });
  });

  it('calls listQueue with selected status when filter changes', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === 'content:moderate' });
    renderPage();

    await waitFor(() => {
      expect(mockModerationApi.listQueue).toHaveBeenCalledWith(
        expect.objectContaining({ status: 'pending' }),
      );
    });

    // Change status filter to "in_review".
    const select = screen.getByRole('combobox');
    fireEvent.change(select, { target: { value: 'in_review' } });

    await waitFor(() => {
      expect(mockModerationApi.listQueue).toHaveBeenCalledWith(
        expect.objectContaining({ status: 'in_review' }),
      );
    });
  });
});
