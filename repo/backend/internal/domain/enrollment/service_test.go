package enrollment_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lms/internal/apperr"
	"lms/internal/domain/enrollment"
	"lms/internal/model"
)

// ── Stubs ─────────────────────────────────────────────────────────────────────

// stubRepo implements enrollment.Repository in memory.
// Enroll is NOT made concurrency-safe in the stub so the race test can verify
// that the repo-layer lock matters; for service-level tests we verify that the
// correct pre-flight errors are returned without needing a race.
type stubRepo struct {
	mu           sync.Mutex
	enrollments  map[string]*model.Enrollment // key: "programID:readerID"
	byID         map[string]*model.Enrollment
	history      map[string][]*model.EnrollmentHistory
	enrollCount  atomic.Int64 // total successful Enroll calls
	maxCapacity  int          // simulated capacity for concurrency test
}

func newStubRepo(capacity int) *stubRepo {
	return &stubRepo{
		enrollments: make(map[string]*model.Enrollment),
		byID:        make(map[string]*model.Enrollment),
		history:     make(map[string][]*model.EnrollmentHistory),
		maxCapacity: capacity,
	}
}

func (r *stubRepo) Enroll(_ context.Context, req enrollment.EnrollRequest) (*model.Enrollment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := req.ProgramID + ":" + req.ReaderID

	// Duplicate guard.
	if _, exists := r.enrollments[key]; exists {
		return nil, &apperr.Conflict{Resource: "enrollment", Message: "reader is already enrolled"}
	}

	// Capacity guard (simulates the SELECT FOR UPDATE check).
	count := 0
	for _, e := range r.enrollments {
		if e.ProgramID == req.ProgramID && e.Status == "confirmed" {
			count++
		}
	}
	if count >= r.maxCapacity {
		return nil, &apperr.Conflict{Resource: "enrollment",
			Message: fmt.Sprintf("program is full (%d/%d seats taken)", count, r.maxCapacity)}
	}

	id := fmt.Sprintf("e-%d", len(r.byID)+1)
	e := &model.Enrollment{
		ID:        id,
		ProgramID: req.ProgramID,
		ReaderID:  req.ReaderID,
		BranchID:  req.BranchID,
		Status:    "confirmed",
	}
	r.enrollments[key] = e
	r.byID[id] = e
	r.enrollCount.Add(1)

	remaining := r.maxCapacity - (count + 1)
	e.RemainingSeats = &remaining
	return e, nil
}

func (r *stubRepo) Drop(_ context.Context, enrollmentID, readerID, branchID, reason, changedBy string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[enrollmentID]
	if !ok {
		return &apperr.NotFound{Resource: "enrollment", ID: enrollmentID}
	}
	if e.Status == "cancelled" {
		return &apperr.Conflict{Resource: "enrollment", Message: "already cancelled"}
	}
	prev := e.Status
	e.Status = "cancelled"
	r.history[enrollmentID] = append(r.history[enrollmentID], &model.EnrollmentHistory{
		EnrollmentID:   enrollmentID,
		PreviousStatus: prev,
		NewStatus:      "cancelled",
		Reason:         strPtr(reason),
	})
	return nil
}

func (r *stubRepo) GetByID(_ context.Context, id, _ string) (*model.Enrollment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return nil, &apperr.NotFound{Resource: "enrollment", ID: id}
	}
	return e, nil
}

func (r *stubRepo) ListByProgram(_ context.Context, programID, _ string, p model.Pagination) (model.PageResult[*model.Enrollment], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var items []*model.Enrollment
	for _, e := range r.enrollments {
		if e.ProgramID == programID {
			items = append(items, e)
		}
	}
	return model.NewPageResult(items, len(items), p), nil
}

func (r *stubRepo) ListByReader(_ context.Context, readerID, _ string, p model.Pagination) (model.PageResult[*model.Enrollment], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var items []*model.Enrollment
	for _, e := range r.enrollments {
		if e.ReaderID == readerID {
			items = append(items, e)
		}
	}
	return model.NewPageResult(items, len(items), p), nil
}

func (r *stubRepo) GetHistory(_ context.Context, enrollmentID string) ([]*model.EnrollmentHistory, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.history[enrollmentID], nil
}

func (r *stubRepo) ConfirmedCount(_ context.Context, programID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, e := range r.enrollments {
		if e.ProgramID == programID && e.Status == "confirmed" {
			count++
		}
	}
	return count, nil
}

// stubProgramLoader returns a configurable program stub.
type stubProgramLoader struct {
	prog  *model.Program
	rules []*model.EnrollmentRule
	prereqs []*model.ProgramPrerequisite
}

func (l *stubProgramLoader) GetByID(_ context.Context, id, _ string) (*model.Program, error) {
	if l.prog == nil || l.prog.ID != id {
		return nil, &apperr.NotFound{Resource: "program", ID: id}
	}
	return l.prog, nil
}
func (l *stubProgramLoader) GetPrerequisites(_ context.Context, _ string) ([]*model.ProgramPrerequisite, error) {
	return l.prereqs, nil
}
func (l *stubProgramLoader) GetEnrollmentRules(_ context.Context, _ string) ([]*model.EnrollmentRule, error) {
	return l.rules, nil
}

// stubReaderLoader returns a configurable reader stub.
type stubReaderLoader struct {
	reader *model.Reader
}

func (l *stubReaderLoader) GetByID(_ context.Context, id, _ string) (*model.Reader, error) {
	if l.reader == nil || l.reader.ID != id {
		return nil, &apperr.NotFound{Resource: "reader", ID: id}
	}
	return l.reader, nil
}

// ── Factories ─────────────────────────────────────────────────────────────────

func publishedProgram(capacity int) *model.Program {
	now := time.Now().UTC()
	return &model.Program{
		ID:                "prog-1",
		BranchID:          "branch-1",
		Title:             "Test Program",
		Capacity:          capacity,
		StartsAt:          now.Add(24 * time.Hour),
		EndsAt:            now.Add(26 * time.Hour),
		Status:            "published",
		EnrollmentChannel: "any",
	}
}

func activeReader(id string) *model.Reader {
	return &model.Reader{
		ID:           id,
		BranchID:     "branch-1",
		ReaderNumber: "R-" + id,
		StatusCode:   "active",
	}
}

func newService(repo *stubRepo, prog *model.Program, rules []*model.EnrollmentRule, prereqs []*model.ProgramPrerequisite, reader *model.Reader) *enrollment.Service {
	return enrollment.NewService(
		repo,
		&stubProgramLoader{prog: prog, rules: rules, prereqs: prereqs},
		&stubReaderLoader{reader: reader},
		nil, // audit logger nil — tested separately
	)
}

func strPtr(s string) *string { return &s }

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestEnroll_HappyPath verifies that a valid enrollment succeeds and returns
// the new enrollment with remaining_seats populated.
func TestEnroll_HappyPath(t *testing.T) {
	repo := newStubRepo(10)
	svc := newService(repo, publishedProgram(10), nil, nil, activeReader("reader-1"))

	e, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1",
		ReaderID:  "reader-1",
		BranchID:  "branch-1",
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if e.Status != "confirmed" {
		t.Errorf("expected status confirmed, got %q", e.Status)
	}
	if e.RemainingSeats == nil {
		t.Error("expected remaining_seats to be set")
	} else if *e.RemainingSeats != 9 {
		t.Errorf("expected remaining_seats=9, got %d", *e.RemainingSeats)
	}
}

// TestEnroll_ClosedWindow_Before verifies that enrollment before the window
// opens returns a closed_window denial.
func TestEnroll_ClosedWindow_Before(t *testing.T) {
	prog := publishedProgram(10)
	opens := time.Now().UTC().Add(1 * time.Hour) // not yet open
	prog.EnrollmentOpensAt = &opens

	repo := newStubRepo(10)
	svc := newService(repo, prog, nil, nil, activeReader("reader-1"))

	_, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	if err == nil {
		t.Fatal("expected closed_window denial")
	}
	var ed *enrollment.EligibilityDenial
	if !errors.As(err, &ed) || ed.Reason != "closed_window" {
		t.Errorf("expected EligibilityDenial{closed_window}, got %T: %v", err, err)
	}
}

// TestEnroll_ClosedWindow_After verifies that enrollment after the window
// closes returns a closed_window denial.
func TestEnroll_ClosedWindow_After(t *testing.T) {
	prog := publishedProgram(10)
	closed := time.Now().UTC().Add(-1 * time.Hour) // already closed
	prog.EnrollmentClosesAt = &closed

	repo := newStubRepo(10)
	svc := newService(repo, prog, nil, nil, activeReader("reader-1"))

	_, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	var ed *enrollment.EligibilityDenial
	if !errors.As(err, &ed) || ed.Reason != "closed_window" {
		t.Errorf("expected EligibilityDenial{closed_window}, got %T: %v", err, err)
	}
}

// TestEnroll_NotPublished verifies that enrolling in a non-published program
// returns a not_published denial.
func TestEnroll_NotPublished(t *testing.T) {
	prog := publishedProgram(10)
	prog.Status = "draft"

	repo := newStubRepo(10)
	svc := newService(repo, prog, nil, nil, activeReader("reader-1"))

	_, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	var ed *enrollment.EligibilityDenial
	if !errors.As(err, &ed) || ed.Reason != "not_published" {
		t.Errorf("expected EligibilityDenial{not_published}, got %T: %v", err, err)
	}
}

// TestEnroll_ReaderIneligible verifies that a frozen reader cannot enroll.
func TestEnroll_ReaderIneligible(t *testing.T) {
	repo := newStubRepo(10)
	frozen := activeReader("reader-2")
	frozen.StatusCode = "frozen"
	svc := newService(repo, publishedProgram(10), nil, nil, frozen)

	_, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-2", BranchID: "branch-1",
	})
	var ed *enrollment.EligibilityDenial
	if !errors.As(err, &ed) || ed.Reason != "reader_ineligible" {
		t.Errorf("expected EligibilityDenial{reader_ineligible}, got %T: %v", err, err)
	}
}

// TestEnroll_DuplicateEnrollment verifies that enrolling the same reader twice
// in the same program returns an apperr.Conflict.
func TestEnroll_DuplicateEnrollment(t *testing.T) {
	repo := newStubRepo(10)
	svc := newService(repo, publishedProgram(10), nil, nil, activeReader("reader-1"))

	// First enrollment — must succeed.
	if _, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	}); err != nil {
		t.Fatalf("first enroll failed: %v", err)
	}

	// Second enrollment — must return Conflict.
	_, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	if err == nil {
		t.Fatal("expected conflict on duplicate enrollment, got nil")
	}
	var ce *apperr.Conflict
	if !errors.As(err, &ce) {
		t.Errorf("expected *apperr.Conflict, got %T: %v", err, err)
	}
}

// TestEnroll_CapacityExhausted verifies that enrollment is rejected when the
// program has no remaining capacity.
func TestEnroll_CapacityExhausted(t *testing.T) {
	const capacity = 2
	repo := newStubRepo(capacity)
	prog := publishedProgram(capacity)
	readers := []*model.Reader{activeReader("r1"), activeReader("r2"), activeReader("r3")}

	svc1 := newService(repo, prog, nil, nil, readers[0])
	svc2 := newService(repo, prog, nil, nil, readers[1])
	svc3 := newService(repo, prog, nil, nil, readers[2])

	if _, err := svc1.Enroll(context.Background(), enrollment.EnrollReaderRequest{ProgramID: "prog-1", ReaderID: "r1", BranchID: "branch-1"}); err != nil {
		t.Fatalf("r1 enroll failed: %v", err)
	}
	if _, err := svc2.Enroll(context.Background(), enrollment.EnrollReaderRequest{ProgramID: "prog-1", ReaderID: "r2", BranchID: "branch-1"}); err != nil {
		t.Fatalf("r2 enroll failed: %v", err)
	}

	// Third enrollment must be rejected.
	_, err := svc3.Enroll(context.Background(), enrollment.EnrollReaderRequest{ProgramID: "prog-1", ReaderID: "r3", BranchID: "branch-1"})
	if err == nil {
		t.Fatal("expected capacity conflict, got nil")
	}
	var ce *apperr.Conflict
	if !errors.As(err, &ce) {
		t.Errorf("expected *apperr.Conflict, got %T: %v", err, err)
	}
}

// TestEnroll_ConcurrencyProtection verifies that the stub repo's mutex prevents
// over-subscription under concurrent goroutines. This test simulates the
// behaviour that SELECT FOR UPDATE enforces at the DB layer.
//
// With capacity=1, exactly 1 of N concurrent goroutines must succeed.
func TestEnroll_ConcurrencyProtection(t *testing.T) {
	const (
		capacity    = 1
		concurrency = 20
	)
	repo := newStubRepo(capacity)
	prog := publishedProgram(capacity)

	var wg sync.WaitGroup
	var successCount atomic.Int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			readerID := fmt.Sprintf("reader-%d", idx)
			// Each goroutine has its own service pointing to the SAME stub repo.
			svc := newService(repo, prog, nil, nil, activeReader(readerID))
			_, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
				ProgramID: "prog-1",
				ReaderID:  readerID,
				BranchID:  "branch-1",
			})
			if err == nil {
				successCount.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if got := successCount.Load(); got != capacity {
		t.Errorf("expected exactly %d successful enrollment(s) for capacity=%d, got %d",
			capacity, capacity, got)
	}
}

// TestEnroll_Blacklisted verifies that a reader matching a blacklist rule is denied.
func TestEnroll_Blacklisted(t *testing.T) {
	blacklistRule := &model.EnrollmentRule{
		ID:         "rule-1",
		ProgramID:  "prog-1",
		RuleType:   "blacklist",
		MatchField: "status_code",
		MatchValue: "active", // blacklists all active readers (contrived but valid for test)
		Reason:     strPtr("test blacklist"),
	}
	repo := newStubRepo(10)
	svc := newService(repo, publishedProgram(10), []*model.EnrollmentRule{blacklistRule}, nil, activeReader("reader-1"))

	_, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	var ed *enrollment.EligibilityDenial
	if !errors.As(err, &ed) || ed.Reason != "blacklisted" {
		t.Errorf("expected EligibilityDenial{blacklisted}, got %T: %v", err, err)
	}
}

// TestEnroll_WhitelistPass verifies that a reader matching a whitelist rule can enroll.
func TestEnroll_WhitelistPass(t *testing.T) {
	whitelistRule := &model.EnrollmentRule{
		ID:         "rule-2",
		ProgramID:  "prog-1",
		RuleType:   "whitelist",
		MatchField: "status_code",
		MatchValue: "active",
	}
	repo := newStubRepo(10)
	svc := newService(repo, publishedProgram(10), []*model.EnrollmentRule{whitelistRule}, nil, activeReader("reader-1"))

	e, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	if err != nil {
		t.Fatalf("expected success for whitelisted reader, got: %v", err)
	}
	if e.Status != "confirmed" {
		t.Errorf("expected status confirmed, got %q", e.Status)
	}
}

// TestEnroll_WhitelistFail verifies that a reader NOT matching a whitelist rule is denied.
func TestEnroll_WhitelistFail(t *testing.T) {
	whitelistRule := &model.EnrollmentRule{
		ID:         "rule-3",
		ProgramID:  "prog-1",
		RuleType:   "whitelist",
		MatchField: "reader_number",
		MatchValue: "SPECIAL-001", // only this reader number is allowed
	}
	repo := newStubRepo(10)
	reader := activeReader("reader-1")
	reader.ReaderNumber = "OTHER-001"
	svc := newService(repo, publishedProgram(10), []*model.EnrollmentRule{whitelistRule}, nil, reader)

	_, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	var ed *enrollment.EligibilityDenial
	if !errors.As(err, &ed) || ed.Reason != "not_whitelisted" {
		t.Errorf("expected EligibilityDenial{not_whitelisted}, got %T: %v", err, err)
	}
}

// TestEnroll_PrerequisiteNotMet verifies that a reader without the required
// completed enrollment is denied.
func TestEnroll_PrerequisiteNotMet(t *testing.T) {
	prereq := &model.ProgramPrerequisite{
		ID:                "prereq-1",
		ProgramID:         "prog-1",
		RequiredProgramID: "prog-prereq",
		Description:       strPtr("Must complete Introduction course"),
	}
	repo := newStubRepo(10)
	svc := newService(repo, publishedProgram(10), nil, []*model.ProgramPrerequisite{prereq}, activeReader("reader-1"))

	_, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	var ed *enrollment.EligibilityDenial
	if !errors.As(err, &ed) || ed.Reason != "prerequisite_not_met" {
		t.Errorf("expected EligibilityDenial{prerequisite_not_met}, got %T: %v", err, err)
	}
}

// TestDrop_HappyPath verifies that dropping an enrollment succeeds and
// records the status change.
func TestDrop_HappyPath(t *testing.T) {
	repo := newStubRepo(10)
	svc := newService(repo, publishedProgram(10), nil, nil, activeReader("reader-1"))

	e, err := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	if err != nil {
		t.Fatalf("enroll failed: %v", err)
	}

	err = svc.Drop(context.Background(), enrollment.DropRequest{
		EnrollmentID: e.ID,
		ReaderID:     "reader-1",
		BranchID:     "branch-1",
		Reason:       "reader request",
		ActorUserID:  "staff-1",
	})
	if err != nil {
		t.Fatalf("drop failed: %v", err)
	}

	// Verify the enrollment is now cancelled.
	dropped := repo.byID[e.ID]
	if dropped.Status != "cancelled" {
		t.Errorf("expected cancelled, got %q", dropped.Status)
	}
}

// TestDrop_AlreadyCancelled verifies that dropping an already-cancelled
// enrollment returns a Conflict error.
func TestDrop_AlreadyCancelled(t *testing.T) {
	repo := newStubRepo(10)
	svc := newService(repo, publishedProgram(10), nil, nil, activeReader("reader-1"))

	e, _ := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})

	// First drop — must succeed.
	_ = svc.Drop(context.Background(), enrollment.DropRequest{
		EnrollmentID: e.ID, ReaderID: "reader-1", BranchID: "branch-1",
	})

	// Second drop — must return Conflict.
	err := svc.Drop(context.Background(), enrollment.DropRequest{
		EnrollmentID: e.ID, ReaderID: "reader-1", BranchID: "branch-1",
	})
	var ce *apperr.Conflict
	if !errors.As(err, &ce) {
		t.Errorf("expected *apperr.Conflict on double-drop, got %T: %v", err, err)
	}
}

// TestDrop_NotFound verifies that dropping a non-existent enrollment returns
// apperr.NotFound.
func TestDrop_NotFound(t *testing.T) {
	repo := newStubRepo(10)
	svc := newService(repo, publishedProgram(10), nil, nil, activeReader("reader-1"))

	err := svc.Drop(context.Background(), enrollment.DropRequest{
		EnrollmentID: "nonexistent",
		ReaderID:     "reader-1",
		BranchID:     "branch-1",
	})
	var nfe *apperr.NotFound
	if !errors.As(err, &nfe) {
		t.Errorf("expected *apperr.NotFound, got %T: %v", err, err)
	}
}

// TestGetHistory_AfterDrop verifies that the enrollment history records the
// drop event.
func TestGetHistory_AfterDrop(t *testing.T) {
	repo := newStubRepo(10)
	svc := newService(repo, publishedProgram(10), nil, nil, activeReader("reader-1"))

	e, _ := svc.Enroll(context.Background(), enrollment.EnrollReaderRequest{
		ProgramID: "prog-1", ReaderID: "reader-1", BranchID: "branch-1",
	})
	_ = svc.Drop(context.Background(), enrollment.DropRequest{
		EnrollmentID: e.ID, ReaderID: "reader-1", BranchID: "branch-1",
		Reason: "changed mind",
	})

	hist, err := svc.GetHistory(context.Background(), e.ID, "branch-1")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(hist) == 0 {
		t.Fatal("expected at least one history entry, got none")
	}
	last := hist[len(hist)-1]
	if last.NewStatus != "cancelled" {
		t.Errorf("expected last history entry to be cancelled, got %q", last.NewStatus)
	}
}
