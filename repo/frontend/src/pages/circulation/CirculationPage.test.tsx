import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import CirculationPage from './CirculationPage';

const { mockUseAuth, mockCirculationApi } = vi.hoisted(() => ({
  mockUseAuth: vi.fn(),
  mockCirculationApi: {
    checkout: vi.fn(),
    return: vi.fn(),
    list: vi.fn(),
    listByCopy: vi.fn(),
    listByReader: vi.fn(),
    getActiveCheckout: vi.fn(),
  },
}));

vi.mock('../../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('../../api/circulation', () => ({
  circulationApi: mockCirculationApi,
}));

const emptyPage = { items: [], total: 0, page: 1, per_page: 20, total_pages: 0 };

const sampleEvent = {
  id: 'ev-1',
  copy_id: 'copy-uuid',
  reader_id: 'reader-uuid',
  branch_id: 'branch-uuid',
  event_type: 'checkout',
  due_date: '2026-05-01',
  created_at: '2026-04-15T10:00:00Z',
};

function renderPage() {
  return render(
    <MemoryRouter>
      <CirculationPage />
    </MemoryRouter>,
  );
}

describe('CirculationPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'circulation:write' || p === 'circulation:read',
      hasRole: () => false,
    });
    mockCirculationApi.list.mockResolvedValue(emptyPage);
    mockCirculationApi.checkout.mockResolvedValue(sampleEvent);
    mockCirculationApi.return.mockResolvedValue(sampleEvent);
  });

  it('renders the Circulation heading', () => {
    renderPage();
    expect(screen.getByText('Circulation')).toBeInTheDocument();
  });

  it('shows Checkout and Return tab buttons for users with circulation:write', () => {
    renderPage();
    // The tab nav has buttons named Checkout and Return.
    const checkoutBtns = screen.getAllByRole('button', { name: /^Checkout$/i });
    expect(checkoutBtns.length).toBeGreaterThanOrEqual(1);
    const returnBtns = screen.getAllByRole('button', { name: /^Return$/i });
    expect(returnBtns.length).toBeGreaterThanOrEqual(1);
  });

  it('shows History tab button for users with circulation:read', () => {
    renderPage();
    expect(screen.getByRole('button', { name: 'History' })).toBeInTheDocument();
  });

  it('hides Checkout and Return tab buttons for read-only users', () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (p: string) => p === 'circulation:read',
      hasRole: () => false,
    });
    renderPage();
    // No Checkout tab button for read-only users.
    // Note: getAllByRole may find 0 or throw; use queryAllByRole for safe check.
    const checkoutBtns = screen.queryAllByRole('button', { name: /^Checkout$/i });
    expect(checkoutBtns.length).toBe(0);
    expect(screen.getByRole('button', { name: 'History' })).toBeInTheDocument();
  });

  it('shows no-permission message for users with neither permission', () => {
    mockUseAuth.mockReturnValue({
      hasPermission: () => false,
      hasRole: () => false,
    });
    renderPage();
    expect(screen.getByText(/do not have permission/i)).toBeInTheDocument();
  });

  it('calls circulationApi.checkout when checkout form is submitted', async () => {
    const { container } = renderPage();

    // The checkout tab is active by default; fill in the form fields.
    fireEvent.change(screen.getByPlaceholderText(/Scan or enter copy barcode/i), {
      target: { value: 'BARCODE-001' },
    });
    fireEvent.change(screen.getByPlaceholderText(/Reader ID or reader number/i), {
      target: { value: 'reader-uuid' },
    });

    // Submit the checkout form directly (avoids button-name ambiguity between tab and submit btn).
    const form = container.querySelector('form');
    if (form) fireEvent.submit(form);

    await waitFor(() => {
      expect(mockCirculationApi.checkout).toHaveBeenCalledWith(
        expect.objectContaining({ barcode: 'BARCODE-001', reader_id: 'reader-uuid' }),
      );
    });
  });

  it('shows success message after successful checkout', async () => {
    const { container } = renderPage();

    fireEvent.change(screen.getByPlaceholderText(/Scan or enter copy barcode/i), {
      target: { value: 'BARCODE-001' },
    });
    fireEvent.change(screen.getByPlaceholderText(/Reader ID or reader number/i), {
      target: { value: 'reader-uuid' },
    });

    const form = container.querySelector('form');
    if (form) fireEvent.submit(form);

    await waitFor(() => {
      expect(screen.getByText(/Checked out/i)).toBeInTheDocument();
    });
  });

  it('calls circulationApi.return when return form is submitted', async () => {
    const { container } = renderPage();

    // Click the Return tab button (the FIRST "Return" element, which is the tab nav button).
    // At this point only the tab nav has a "Return" button; the form is not yet mounted.
    const returnTabBtn = screen.getAllByRole('button', { name: /^Return$/i })[0];
    fireEvent.click(returnTabBtn);

    // Now ReturnTab is mounted; fill in the barcode.
    await waitFor(() => {
      expect(screen.getAllByPlaceholderText(/Scan or enter copy barcode/i).length).toBeGreaterThan(0);
    });

    const barcodeInputs = screen.getAllByPlaceholderText(/Scan or enter copy barcode/i);
    fireEvent.change(barcodeInputs[barcodeInputs.length - 1], { target: { value: 'RETURN-001' } });

    // Submit the return form.
    const forms = container.querySelectorAll('form');
    if (forms.length > 0) fireEvent.submit(forms[forms.length - 1]);

    await waitFor(() => {
      expect(mockCirculationApi.return).toHaveBeenCalledWith(
        expect.objectContaining({ barcode: 'RETURN-001' }),
      );
    });
  });

  it('shows circulation history in History tab', async () => {
    mockCirculationApi.list.mockResolvedValue({
      items: [sampleEvent],
      total: 1,
      page: 1,
      per_page: 20,
      total_pages: 1,
    });

    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'History' }));

    await waitFor(() => {
      expect(screen.getByText('copy-uuid')).toBeInTheDocument();
    });
  });

  it('shows "No circulation events found" when history is empty', async () => {
    mockCirculationApi.list.mockResolvedValue(emptyPage);
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'History' }));

    await waitFor(() => {
      expect(screen.getByText(/No circulation events found/i)).toBeInTheDocument();
    });
  });
});
