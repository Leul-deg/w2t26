package imports_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lms/internal/apperr"
	"lms/internal/domain/imports"
	"lms/internal/model"
)

// ── Stub repository ───────────────────────────────────────────────────────────

type stubRepo struct {
	jobs map[string]*model.ImportJob
	rows map[string][]*model.ImportRow
}

func newStubRepo() *stubRepo {
	return &stubRepo{
		jobs: make(map[string]*model.ImportJob),
		rows: make(map[string][]*model.ImportRow),
	}
}

func (r *stubRepo) CreateJob(_ context.Context, job *model.ImportJob) error {
	job.ID = "job-001"
	r.jobs[job.ID] = job
	return nil
}

func (r *stubRepo) GetJob(_ context.Context, id, _ string) (*model.ImportJob, error) {
	j, ok := r.jobs[id]
	if !ok {
		return nil, &apperr.NotFound{Resource: "import_job", ID: id}
	}
	return j, nil
}

func (r *stubRepo) UpdateJobStatus(_ context.Context, id, status string, errCount int, errSummary any) error {
	j, ok := r.jobs[id]
	if !ok {
		return errors.New("job not found")
	}
	j.Status = status
	j.ErrorCount = errCount
	j.ErrorSummary = errSummary
	return nil
}

func (r *stubRepo) ListJobs(_ context.Context, _ string, p model.Pagination) (model.PageResult[*model.ImportJob], error) {
	var items []*model.ImportJob
	for _, j := range r.jobs {
		items = append(items, j)
	}
	return model.NewPageResult(items, len(items), p), nil
}

func (r *stubRepo) CreateRows(_ context.Context, jobID string, rows []*model.ImportRow) error {
	for _, row := range rows {
		row.JobID = jobID
	}
	r.rows[jobID] = append(r.rows[jobID], rows...)
	return nil
}

func (r *stubRepo) ListRows(_ context.Context, jobID string, p model.Pagination) (model.PageResult[*model.ImportRow], error) {
	rows := r.rows[jobID]
	total := len(rows)
	start := p.Offset()
	if start >= total {
		return model.NewPageResult([]*model.ImportRow{}, total, p), nil
	}
	end := start + p.Limit()
	if end > total {
		end = total
	}
	return model.NewPageResult(rows[start:end], total, p), nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// newSvc creates a Service backed by a stub repo and NO pool (commit tests
// that need a real pool are integration tests; unit tests stop before commit).
func newSvc(repo imports.Repository) *imports.Service {
	return imports.NewService(repo, nil, nil)
}

func makeCSV(rows ...string) []byte {
	return []byte(strings.Join(rows, "\n") + "\n")
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestUpload_ValidationFailure_UnknownStatus verifies that a row with an
// unknown status_code is flagged as invalid and the job status is "failed".
// The entire import cannot be committed when error_count > 0.
func TestUpload_ValidationFailure_UnknownStatus(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	csv := makeCSV(
		"reader_number,first_name,last_name,status_code",
		"R001,Alice,Smith,active",
		"R002,Bob,Jones,invalid_status", // <-- bad status
	)
	result, err := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "readers",
		FileName:    "test.csv",
		Data:        csv,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasErrors {
		t.Error("expected HasErrors=true")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one row error")
	}
	if result.Job.Status != "failed" {
		t.Errorf("expected job status failed, got %q", result.Job.Status)
	}
}

// TestUpload_ValidationFailure_MissingRequiredField verifies that a row with a
// blank first_name is rejected with a clear field-level error.
func TestUpload_ValidationFailure_MissingRequiredField(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	csv := makeCSV(
		"reader_number,first_name,last_name",
		"R001,,Smith", // first_name blank
	)
	result, err := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "readers",
		FileName:    "missing_first.csv",
		Data:        csv,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasErrors {
		t.Error("expected HasErrors=true")
	}
	found := false
	for _, e := range result.Errors {
		if e.Field == "first_name" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a first_name error; got: %+v", result.Errors)
	}
}

// TestUpload_MissingRequiredHeader verifies that a CSV missing the "title"
// column for a holdings import is rejected at the structure level (status=failed).
func TestUpload_MissingRequiredHeader(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	csv := makeCSV(
		"author,isbn", // no title column
		"Tolkien,978-0",
	)
	result, err := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "holdings",
		FileName:    "bad_headers.csv",
		Data:        csv,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Job.Status != "failed" {
		t.Errorf("expected status failed, got %q", result.Job.Status)
	}
}

// TestUpload_DuplicateReaderNumberInFile verifies that two rows in the same CSV
// file with the same reader_number are both flagged (deduplication within file).
func TestUpload_DuplicateReaderNumberInFile(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	csv := makeCSV(
		"reader_number,first_name,last_name",
		"R001,Alice,Smith",
		"R001,Bob,Jones", // duplicate reader_number
	)
	result, err := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "readers",
		FileName:    "dup_numbers.csv",
		Data:        csv,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasErrors {
		t.Error("expected HasErrors=true for duplicate reader_number")
	}
	found := false
	for _, e := range result.Errors {
		if e.Field == "reader_number" && strings.Contains(e.Message, "duplicate") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate reader_number error; got: %+v", result.Errors)
	}
}

// TestUpload_DuplicateISBNInFile verifies dedup enforcement for holdings.
func TestUpload_DuplicateISBNInFile(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	csv := makeCSV(
		"title,isbn",
		"Book A,978-0000000001",
		"Book B,978-0000000001", // same ISBN
	)
	result, err := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "holdings",
		FileName:    "dup_isbn.csv",
		Data:        csv,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasErrors {
		t.Error("expected HasErrors=true for duplicate ISBN")
	}
	found := false
	for _, e := range result.Errors {
		if e.Field == "isbn" && strings.Contains(e.Message, "duplicate") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate isbn error; got: %+v", result.Errors)
	}
}

// TestCommit_BlockedWhenErrorsPresent verifies that CommitJob returns a
// validation error when the job has error_count > 0 — the "no partial import"
// guarantee is enforced before a transaction is ever opened.
func TestCommit_BlockedWhenErrorsPresent(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	// Stage a job with errors via upload.
	csv := makeCSV(
		"reader_number,first_name,last_name",
		"R001,,Smith", // invalid row
	)
	result, _ := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "readers",
		FileName:    "bad.csv",
		Data:        csv,
	})

	// Attempt commit — must fail without opening a DB transaction.
	_, err := svc.CommitJob(context.Background(), result.Job.ID, "branch-1", "actor-1")
	if err == nil {
		t.Fatal("expected error from CommitJob when errors present, got nil")
	}
	// Should be a Validation error about error_count.
	var ve *apperr.Validation
	if !errors.As(err, &ve) {
		t.Errorf("expected *apperr.Validation, got %T: %v", err, err)
	}
}

// TestCommit_BlockedWhenNotPreviewReady verifies that CommitJob rejects a job
// that is not in preview_ready status (e.g. already committed).
func TestCommit_BlockedWhenNotPreviewReady(t *testing.T) {
	repo := newStubRepo()
	// Manually inject a committed job.
	repo.jobs["committed-job"] = &model.ImportJob{
		ID:       "committed-job",
		BranchID: "branch-1",
		Status:   "committed",
	}

	svc := newSvc(repo)
	_, err := svc.CommitJob(context.Background(), "committed-job", "branch-1", "actor-1")
	if err == nil {
		t.Fatal("expected error when committing an already-committed job")
	}
	var ve *apperr.Validation
	if !errors.As(err, &ve) {
		t.Errorf("expected *apperr.Validation, got %T: %v", err, err)
	}
}

// TestRollback_UpdatesStatus verifies that RollbackJob transitions a
// preview_ready job to rolled_back without any DB insert.
func TestRollback_UpdatesStatus(t *testing.T) {
	repo := newStubRepo()
	repo.jobs["job-pr"] = &model.ImportJob{
		ID:         "job-pr",
		BranchID:   "branch-1",
		Status:     "preview_ready",
		ErrorCount: 0,
	}
	svc := newSvc(repo)

	job, err := svc.RollbackJob(context.Background(), "job-pr", "branch-1", "actor-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Status != "rolled_back" {
		t.Errorf("expected rolled_back, got %q", job.Status)
	}
	// Confirm the repo was updated.
	persisted := repo.jobs["job-pr"]
	if persisted.Status != "rolled_back" {
		t.Errorf("repo not updated; status=%q", persisted.Status)
	}
}

// TestRollback_BlockedWhenAlreadyCommitted verifies that a committed job cannot
// be rolled back.
func TestRollback_BlockedWhenAlreadyCommitted(t *testing.T) {
	repo := newStubRepo()
	repo.jobs["job-done"] = &model.ImportJob{
		ID:       "job-done",
		BranchID: "branch-1",
		Status:   "committed",
	}
	svc := newSvc(repo)

	_, err := svc.RollbackJob(context.Background(), "job-done", "branch-1", "actor-1")
	if err == nil {
		t.Fatal("expected error rolling back committed job")
	}
}

// TestUpload_ValidReaders_NoErrors verifies that a well-formed reader CSV
// produces preview_ready status and zero errors.
func TestUpload_ValidReaders_NoErrors(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	csv := makeCSV(
		"reader_number,first_name,last_name,status_code",
		"R001,Alice,Smith,active",
		"R002,Bob,Jones,frozen",
	)
	result, err := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "readers",
		FileName:    "good.csv",
		Data:        csv,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasErrors {
		t.Errorf("expected no errors; got: %+v", result.Errors)
	}
	if result.Job.Status != "preview_ready" {
		t.Errorf("expected preview_ready, got %q", result.Job.Status)
	}
	if result.Job.ErrorCount != 0 {
		t.Errorf("expected error_count=0, got %d", result.Job.ErrorCount)
	}
}

// TestUpload_InvalidPublicationYear verifies year validation for holdings.
func TestUpload_InvalidPublicationYear(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	csv := makeCSV(
		"title,publication_year",
		"My Book,not-a-year",
	)
	result, err := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "holdings",
		FileName:    "bad_year.csv",
		Data:        csv,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasErrors {
		t.Error("expected HasErrors=true for invalid publication_year")
	}
	found := false
	for _, e := range result.Errors {
		if e.Field == "publication_year" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected publication_year error; got: %+v", result.Errors)
	}
}

// TestErrorCSV_GeneratesDownloadableReport verifies that ErrorCSV returns a
// non-empty CSV when the job has errors.
func TestErrorCSV_GeneratesDownloadableReport(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	csv := makeCSV(
		"reader_number,first_name,last_name",
		"R001,,Smith",
	)
	result, _ := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "readers",
		FileName:    "err.csv",
		Data:        csv,
	})

	data, fileName, err := svc.ErrorCSV(context.Background(), result.Job.ID, "branch-1")
	if err != nil {
		t.Fatalf("ErrorCSV failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty error CSV")
	}
	if !strings.Contains(fileName, ".csv") {
		t.Errorf("unexpected file name (expected .csv extension): %q", fileName)
	}
	// CSV should contain the header row.
	if !strings.Contains(string(data), "row_number") {
		t.Error("expected CSV to contain header row_number")
	}
}

// TestTemplateCSV verifies that template CSVs are generated for all supported types.
func TestTemplateCSV(t *testing.T) {
	for _, importType := range []string{"readers", "holdings"} {
		data, fileName, err := imports.TemplateCSV(importType)
		if err != nil {
			t.Errorf("TemplateCSV(%q) failed: %v", importType, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("TemplateCSV(%q) returned empty data", importType)
		}
		if !strings.HasSuffix(fileName, ".csv") {
			t.Errorf("TemplateCSV(%q): unexpected file name %q", importType, fileName)
		}
	}
}

// TestUpload_UnsupportedImportType verifies that an unknown import_type is
// rejected before any CSV parsing occurs.
func TestUpload_UnsupportedImportType(t *testing.T) {
	repo := newStubRepo()
	svc := newSvc(repo)

	_, err := svc.UploadAndParse(context.Background(), imports.UploadRequest{
		BranchID:    "branch-1",
		ActorUserID: "actor-1",
		ImportType:  "purchases", // not supported
		FileName:    "bad.csv",
		Data:        []byte("col1,col2\nval1,val2"),
	})
	if err == nil {
		t.Fatal("expected error for unsupported import type")
	}
	var ve *apperr.Validation
	if !errors.As(err, &ve) {
		t.Errorf("expected *apperr.Validation, got %T", err)
	}
}
