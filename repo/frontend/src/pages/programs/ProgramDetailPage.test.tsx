import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import ProgramDetailPage from './ProgramDetailPage';

const { mockUseAuth, mockProgramsApi } = vi.hoisted(() => ({
  mockUseAuth: vi.fn(),
  mockProgramsApi: {
    get: vi.fn(),
    getSeats: vi.fn(),
    updateStatus: vi.fn(),
    addPrerequisite: vi.fn(),
    removePrerequisite: vi.fn(),
    addRule: vi.fn(),
    removeRule: vi.fn(),
    listEnrollmentsByProgram: vi.fn(),
    listEnrollmentsByReader: vi.fn(),
    enroll: vi.fn(),
    dropEnrollment: vi.fn(),
    getEnrollmentHistory: vi.fn(),
  },
}));

vi.mock('../../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('../../api/programs', () => ({
  programsApi: mockProgramsApi,
}));

const baseProgram = {
  id: 'prog-1',
  branch_id: 'branch-1',
  title: 'Story Time',
  description: 'Weekly reading program',
  capacity: 20,
  starts_at: '2026-04-10T10:00:00Z',
  ends_at: '2026-04-10T11:00:00Z',
  status: 'draft',
  enrollment_channel: 'any',
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-01T00:00:00Z',
  prerequisites: [],
  enrollment_rules: [],
};

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/programs/prog-1']}>
      <Routes>
        <Route path="/programs/:id" element={<ProgramDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ProgramDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      hasPermission: (perm: string) => perm === 'programs:write',
    });
    mockProgramsApi.get.mockResolvedValue(baseProgram);
    mockProgramsApi.getSeats.mockResolvedValue({ remaining_seats: 12 });
    mockProgramsApi.listEnrollmentsByProgram.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      per_page: 20,
      total_pages: 1,
    });
    mockProgramsApi.updateStatus.mockResolvedValue({ status: 'published' });
    mockProgramsApi.addPrerequisite.mockResolvedValue({});
    mockProgramsApi.addRule.mockResolvedValue({});
  });

  it('allows staff to update program status', async () => {
    renderPage();

    await waitFor(() => expect(screen.getByText('Program controls')).toBeInTheDocument());

    fireEvent.change(screen.getByDisplayValue('draft'), { target: { value: 'published' } });
    fireEvent.click(screen.getByRole('button', { name: /save status/i }));

    await waitFor(() => {
      expect(mockProgramsApi.updateStatus).toHaveBeenCalledWith('prog-1', 'published');
    });
  });

  it('allows staff to add prerequisites and enrollment rules', async () => {
    renderPage();

    await waitFor(() => expect(screen.getByText('Prerequisites')).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText('Program UUID'), { target: { value: 'prog-2' } });
    fireEvent.change(screen.getByPlaceholderText('Optional explanatory note'), { target: { value: 'Must complete intro' } });
    fireEvent.click(screen.getByRole('button', { name: /add prerequisite/i }));

    await waitFor(() => {
      expect(mockProgramsApi.addPrerequisite).toHaveBeenCalledWith('prog-1', 'prog-2', 'Must complete intro');
    });

    fireEvent.change(screen.getByPlaceholderText('e.g. branch_id'), { target: { value: 'branch_id' } });
    fireEvent.change(screen.getByPlaceholderText('Value to compare'), { target: { value: 'branch-1' } });
    fireEvent.change(screen.getByPlaceholderText('Optional explanation'), { target: { value: 'Local branch only' } });
    fireEvent.click(screen.getByRole('button', { name: /add rule/i }));

    await waitFor(() => {
      expect(mockProgramsApi.addRule).toHaveBeenCalledWith('prog-1', {
        rule_type: 'whitelist',
        match_field: 'branch_id',
        match_value: 'branch-1',
        reason: 'Local branch only',
      });
    });
  });
});
