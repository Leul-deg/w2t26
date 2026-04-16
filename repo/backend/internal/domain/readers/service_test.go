package readers_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"lms/internal/domain/readers"
	"lms/internal/model"
)

// ── stub repository ───────────────────────────────────────────────────────────

type stubReaderRepo struct {
	readers map[string]*model.Reader
	nextID  int
}

func newStubReaderRepo() *stubReaderRepo {
	return &stubReaderRepo{readers: make(map[string]*model.Reader)}
}

func (r *stubReaderRepo) Create(_ context.Context, reader *model.Reader) error {
	r.nextID++
	reader.ID = fmt.Sprintf("reader-%03d", r.nextID)
	r.readers[reader.ID] = reader
	return nil
}

func (r *stubReaderRepo) GetByID(_ context.Context, id, _ string) (*model.Reader, error) {
	if reader, ok := r.readers[id]; ok {
		return reader, nil
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (r *stubReaderRepo) GetByReaderNumber(_ context.Context, number, _ string) (*model.Reader, error) {
	for _, reader := range r.readers {
		if reader.ReaderNumber == number {
			return reader, nil
		}
	}
	return nil, fmt.Errorf("not found: %s", number)
}

func (r *stubReaderRepo) Update(_ context.Context, reader *model.Reader) error {
	r.readers[reader.ID] = reader
	return nil
}

func (r *stubReaderRepo) UpdateStatus(_ context.Context, id, _, statusCode string) error {
	if reader, ok := r.readers[id]; ok {
		reader.StatusCode = statusCode
	}
	return nil
}

func (r *stubReaderRepo) List(_ context.Context, _ string, _ readers.ReaderFilter, _ model.Pagination) (model.PageResult[*model.Reader], error) {
	items := make([]*model.Reader, 0, len(r.readers))
	for _, reader := range r.readers {
		items = append(items, reader)
	}
	return model.NewPageResult(items, len(items), model.Pagination{Page: 1, PerPage: 20}), nil
}

func (r *stubReaderRepo) ListStatuses(_ context.Context) ([]*model.ReaderStatus, error) {
	return []*model.ReaderStatus{}, nil
}

func (r *stubReaderRepo) GetLoanHistory(_ context.Context, _, _ string, _ model.Pagination) (model.PageResult[*readers.LoanHistoryItem], error) {
	return model.PageResult[*readers.LoanHistoryItem]{}, nil
}

func (r *stubReaderRepo) GetCurrentHoldings(_ context.Context, _, _ string) ([]*readers.LoanHistoryItem, error) {
	return nil, nil
}

// ── Create validation ─────────────────────────────────────────────────────────

func TestReaders_Create_MissingFirstName(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	_, err := svc.Create(context.Background(), readers.CreateRequest{
		LastName: "Smith",
		BranchID: "branch-1",
	})
	if err == nil {
		t.Fatal("expected validation error for missing first_name")
	}
}

func TestReaders_Create_MissingLastName(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	_, err := svc.Create(context.Background(), readers.CreateRequest{
		FirstName: "Jane",
		BranchID:  "branch-1",
	})
	if err == nil {
		t.Fatal("expected validation error for missing last_name")
	}
}

func TestReaders_Create_MissingBranch(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	_, err := svc.Create(context.Background(), readers.CreateRequest{
		FirstName: "Jane",
		LastName:  "Smith",
	})
	if err == nil {
		t.Fatal("expected validation error for missing branch_id")
	}
}

// TestReaders_Create_Success verifies that a valid request creates a reader
// with status "active" and a generated reader number.
func TestReaders_Create_Success(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	r, err := svc.Create(context.Background(), readers.CreateRequest{
		FirstName: "Jane",
		LastName:  "Smith",
		BranchID:  "branch-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.StatusCode != "active" {
		t.Errorf("expected status_code=active, got %q", r.StatusCode)
	}
	if r.ReaderNumber == "" {
		t.Error("expected non-empty reader_number to be generated")
	}
}

// TestReaders_Create_ExplicitReaderNumber verifies that an explicit reader number is used.
func TestReaders_Create_ExplicitReaderNumber(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	r, err := svc.Create(context.Background(), readers.CreateRequest{
		FirstName:    "Bob",
		LastName:     "Jones",
		BranchID:     "branch-1",
		ReaderNumber: "RN-EXPLICIT",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ReaderNumber != "RN-EXPLICIT" {
		t.Errorf("expected reader_number=RN-EXPLICIT, got %q", r.ReaderNumber)
	}
}

// ── GetByID validation ────────────────────────────────────────────────────────

func TestReaders_GetByID_NonUUID(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	_, err := svc.GetByID(context.Background(), "not-a-uuid", "branch-1")
	if err == nil {
		t.Fatal("expected NotFound error for non-UUID id")
	}
}

// ── Update validation ─────────────────────────────────────────────────────────

func TestReaders_Update_NonUUID(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	_, err := svc.Update(context.Background(), "not-a-uuid", "branch-1", readers.UpdateRequest{
		FirstName: "Jane",
		LastName:  "Smith",
	})
	if err == nil {
		t.Fatal("expected NotFound error for non-UUID id")
	}
}

func TestReaders_Update_EmptyFirstName(t *testing.T) {
	repo := newStubReaderRepo()
	r := &model.Reader{
		ID:           "00000000-0000-0000-0000-000000000001",
		FirstName:    "Jane",
		LastName:     "Smith",
		BranchID:     "branch-1",
		RegisteredAt: time.Now(),
	}
	repo.readers[r.ID] = r

	svc := readers.NewService(repo, nil, nil)
	_, err := svc.Update(context.Background(), r.ID, "branch-1", readers.UpdateRequest{
		FirstName: "",
		LastName:  "Smith",
	})
	if err == nil {
		t.Fatal("expected validation error for empty first_name")
	}
}

func TestReaders_Update_EmptyLastName(t *testing.T) {
	repo := newStubReaderRepo()
	r := &model.Reader{
		ID:           "00000000-0000-0000-0000-000000000001",
		FirstName:    "Jane",
		LastName:     "Smith",
		BranchID:     "branch-1",
		RegisteredAt: time.Now(),
	}
	repo.readers[r.ID] = r

	svc := readers.NewService(repo, nil, nil)
	_, err := svc.Update(context.Background(), r.ID, "branch-1", readers.UpdateRequest{
		FirstName: "Jane",
		LastName:  "",
	})
	if err == nil {
		t.Fatal("expected validation error for empty last_name")
	}
}

// ── UpdateStatus validation ───────────────────────────────────────────────────

func TestReaders_UpdateStatus_NonUUID(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	err := svc.UpdateStatus(context.Background(), "not-a-uuid", "branch-1", "user-1", "active")
	if err == nil {
		t.Fatal("expected NotFound error for non-UUID id")
	}
}

func TestReaders_UpdateStatus_InvalidStatus(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	err := svc.UpdateStatus(context.Background(), "00000000-0000-0000-0000-000000000001", "branch-1", "user-1", "bogus")
	if err == nil {
		t.Fatal("expected validation error for invalid status_code")
	}
}

func TestReaders_UpdateStatus_ValidStatuses(t *testing.T) {
	for _, status := range []string{"active", "frozen", "blacklisted", "pending_verification"} {
		repo := newStubReaderRepo()
		r := &model.Reader{
			ID:           "00000000-0000-0000-0000-000000000001",
			StatusCode:   "active",
			RegisteredAt: time.Now(),
		}
		repo.readers[r.ID] = r

		svc := readers.NewService(repo, nil, nil)
		err := svc.UpdateStatus(context.Background(), r.ID, "branch-1", "user-1", status)
		if err != nil {
			t.Errorf("status %q should be valid, got error: %v", status, err)
		}
	}
}

// ── GetLoanHistory validation ─────────────────────────────────────────────────

func TestReaders_GetLoanHistory_NonUUID(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	_, err := svc.GetLoanHistory(context.Background(), "not-a-uuid", "branch-1", model.Pagination{Page: 1, PerPage: 20})
	if err == nil {
		t.Fatal("expected NotFound error for non-UUID reader id")
	}
}

// ── GetCurrentHoldings validation ─────────────────────────────────────────────

func TestReaders_GetCurrentHoldings_NonUUID(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	_, err := svc.GetCurrentHoldings(context.Background(), "not-a-uuid", "branch-1")
	if err == nil {
		t.Fatal("expected NotFound error for non-UUID reader id")
	}
}

// ── Encryption disabled (nil key) ─────────────────────────────────────────────

// TestReaders_Create_NilKeyStoresPlaintext verifies that with a nil encryption
// key (dev/test mode), sensitive fields are stored as-is (no encryption error).
func TestReaders_Create_NilKeyStoresPlaintext(t *testing.T) {
	svc := readers.NewService(newStubReaderRepo(), nil, nil)
	email := "test@example.com"
	r, err := svc.Create(context.Background(), readers.CreateRequest{
		FirstName:    "Jane",
		LastName:     "Smith",
		BranchID:     "branch-1",
		ContactEmail: &email,
	})
	if err != nil {
		t.Fatalf("unexpected error with nil encryption key: %v", err)
	}
	// With nil key the value should be passed through as plaintext.
	if r.ContactEmailEnc == nil || *r.ContactEmailEnc != email {
		t.Errorf("expected contact_email_enc to be %q (plaintext), got %v", email, r.ContactEmailEnc)
	}
}
