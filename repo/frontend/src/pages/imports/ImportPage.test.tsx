import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import ImportPage from './ImportPage';

const { mockUseAuth, mockImportsApi } = vi.hoisted(() => ({
  mockUseAuth: vi.fn(),
  mockImportsApi: {
    upload: vi.fn(),
    getPreview: vi.fn(),
    commit: vi.fn(),
    rollback: vi.fn(),
    errorsFileUrl: vi.fn(),
    templateFileUrl: vi.fn(),
  },
}));

vi.mock('../../auth/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('../../api/imports', () => ({
  importsApi: mockImportsApi,
}));

const previewJob = {
  id: 'job-1',
  branch_id: 'branch-1',
  import_type: 'readers',
  status: 'preview_ready',
  file_name: 'readers.xlsx',
  row_count: 10,
  error_count: 0,
  error_summary: [],
  valid_row_count: 9,
  invalid_row_count: 1,
  completeness_percent: 90,
  completeness_threshold_percent: 100,
  meets_completeness_threshold: false,
  uploaded_by: 'u1',
  uploaded_at: '2026-04-08T00:00:00Z',
};

function renderPage() {
  return render(
    <MemoryRouter>
      <ImportPage />
    </MemoryRouter>,
  );
}

describe('ImportPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockImportsApi.templateFileUrl.mockReturnValue('/template');
    mockImportsApi.errorsFileUrl.mockReturnValue('/errors');
    mockImportsApi.upload.mockResolvedValue(previewJob);
    mockImportsApi.getPreview.mockResolvedValue({
      job: previewJob,
      rows: { items: [], total: 0, page: 1, per_page: 20, total_pages: 1 },
    });
  });

  it('shows completeness summary after upload preview', async () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (perm: string) => perm === 'imports:create' || perm === 'imports:preview' || perm === 'imports:commit',
    });

    const { container } = renderPage();

    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    expect(fileInput).toBeTruthy();
    const file = new File(['id,name'], 'readers.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    });
    fireEvent.change(fileInput, { target: { files: [file] } });
    fireEvent.click(screen.getByRole('button', { name: /upload & validate/i }));

    await waitFor(() => expect(screen.getByText(/completeness threshold:/i)).toBeInTheDocument());
    expect(screen.getByText('90.0%')).toBeInTheDocument();
    expect(screen.getByText(/threshold not met/i)).toBeInTheDocument();
  });

  it('hides commit and rollback actions without imports:commit', async () => {
    mockUseAuth.mockReturnValue({
      hasPermission: (perm: string) => perm === 'imports:create' || perm === 'imports:preview',
    });

    const { container } = renderPage();

    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement;
    expect(fileInput).toBeTruthy();
    const file = new File(['id,name'], 'readers.csv', { type: 'text/csv' });
    fireEvent.change(fileInput, { target: { files: [file] } });
    fireEvent.click(screen.getByRole('button', { name: /upload & validate/i }));

    await waitFor(() => expect(screen.getByText(/validation summary/i)).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /commit import/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /cancel import/i })).not.toBeInTheDocument();
  });
});
