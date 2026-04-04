package appeals_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"lms/internal/domain/appeals"
	"lms/internal/model"
)

// ── stub repository ───────────────────────────────────────────────────────────

type stubAppealsRepo struct {
	appeals       map[string]*model.Appeal
	arbitrations  map[string]*model.AppealArbitration
	nextID        int
}

func newStubRepo() *stubAppealsRepo {
	return &stubAppealsRepo{
		appeals:      make(map[string]*model.Appeal),
		arbitrations: make(map[string]*model.AppealArbitration),
	}
}

func (r *stubAppealsRepo) Create(_ context.Context, a *model.Appeal) error {
	r.nextID++
	a.ID = fmt.Sprintf("appeal-%03d", r.nextID)
	a.SubmittedAt = time.Now()
	a.UpdatedAt = time.Now()
	r.appeals[a.ID] = a
	return nil
}

func (r *stubAppealsRepo) GetByID(_ context.Context, id, _ string) (*model.Appeal, error) {
	if a, ok := r.appeals[id]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (r *stubAppealsRepo) UpdateStatus(_ context.Context, id, status string) error {
	if a, ok := r.appeals[id]; ok {
		a.Status = status
		a.UpdatedAt = time.Now()
		return nil
	}
	return fmt.Errorf("not found: %s", id)
}

func (r *stubAppealsRepo) List(_ context.Context, _ string, _ appeals.AppealFilter, _ model.Pagination) (model.PageResult[*model.Appeal], error) {
	items := make([]*model.Appeal, 0, len(r.appeals))
	for _, a := range r.appeals {
		items = append(items, a)
	}
	return model.NewPageResult(items, len(items), model.Pagination{Page: 1, PerPage: 20}), nil
}

func (r *stubAppealsRepo) Arbitrate(_ context.Context, arb *model.AppealArbitration) error {
	r.nextID++
	arb.ID = fmt.Sprintf("arb-%03d", r.nextID)
	arb.DecidedAt = time.Now()
	r.arbitrations[arb.AppealID] = arb
	return nil
}

func (r *stubAppealsRepo) GetArbitration(_ context.Context, appealID string) (*model.AppealArbitration, error) {
	if arb, ok := r.arbitrations[appealID]; ok {
		return arb, nil
	}
	return nil, fmt.Errorf("no arbitration for appeal: %s", appealID)
}

// ── TestSubmit_HappyPath ──────────────────────────────────────────────────────

func TestSubmit_HappyPath(t *testing.T) {
	svc := appeals.NewService(newStubRepo(), nil)

	a, err := svc.Submit(context.Background(), appeals.SubmitRequest{
		BranchID:    "branch-001",
		ReaderID:    "reader-001",
		AppealType:  "enrollment_denial",
		Reason:      "I was incorrectly excluded from the program",
		ActorUserID: "staff-001",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if a.Status != "submitted" {
		t.Errorf("expected status=submitted, got %q", a.Status)
	}
	if a.ID == "" {
		t.Error("expected non-empty ID")
	}
}

// ── TestSubmit_MissingReason_ReturnsValidation ────────────────────────────────

func TestSubmit_MissingReason_ReturnsValidation(t *testing.T) {
	svc := appeals.NewService(newStubRepo(), nil)
	_, err := svc.Submit(context.Background(), appeals.SubmitRequest{
		BranchID:   "branch-001",
		ReaderID:   "reader-001",
		AppealType: "other",
		Reason:     "", // missing
	})
	if err == nil {
		t.Fatal("expected validation error for empty reason")
	}
}

// ── TestSubmit_InvalidType_ReturnsValidation ──────────────────────────────────

func TestSubmit_InvalidType_ReturnsValidation(t *testing.T) {
	svc := appeals.NewService(newStubRepo(), nil)
	_, err := svc.Submit(context.Background(), appeals.SubmitRequest{
		BranchID:   "branch-001",
		ReaderID:   "reader-001",
		AppealType: "not_a_real_type",
		Reason:     "some reason",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid appeal_type")
	}
}

// ── TestStateTransition_SubmittedToReviewToResolved ───────────────────────────

func TestStateTransition_SubmittedToReviewToResolved(t *testing.T) {
	repo := newStubRepo()
	svc := appeals.NewService(repo, nil)

	// Step 1: Submit.
	a, err := svc.Submit(context.Background(), appeals.SubmitRequest{
		BranchID:   "branch-001",
		ReaderID:   "reader-001",
		AppealType: "account_suspension",
		Reason:     "wrongful suspension",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if a.Status != "submitted" {
		t.Fatalf("expected submitted, got %s", a.Status)
	}

	// Step 2: Review.
	a, err = svc.Review(context.Background(), a.ID, "branch-001", "moderator-001")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if a.Status != "under_review" {
		t.Fatalf("expected under_review, got %s", a.Status)
	}

	// Step 3: Arbitrate (upheld → resolved).
	appealID := a.ID
	a, arb, err := svc.Arbitrate(context.Background(), appeals.ArbitrateRequest{
		AppealID:      appealID,
		BranchID:      "branch-001",
		Decision:      "upheld",
		DecisionNotes: "Suspension was not justified by the evidence",
		ActorUserID:   "admin-001",
	})
	if err != nil {
		t.Fatalf("Arbitrate: %v", err)
	}
	if a.Status != "resolved" {
		t.Errorf("expected status=resolved after upheld, got %q", a.Status)
	}
	if arb.Decision != "upheld" {
		t.Errorf("expected arbitration decision=upheld, got %q", arb.Decision)
	}
	if arb.ID == "" {
		t.Error("expected non-empty arbitration ID")
	}
}

// ── TestStateTransition_SubmittedToDismissed ──────────────────────────────────

func TestStateTransition_SubmittedToDismissed(t *testing.T) {
	repo := newStubRepo()
	svc := appeals.NewService(repo, nil)

	a, _ := svc.Submit(context.Background(), appeals.SubmitRequest{
		BranchID: "b1", ReaderID: "r1", AppealType: "other", Reason: "test",
	})
	a, _ = svc.Review(context.Background(), a.ID, "b1", "mod-1")

	a, arb, err := svc.Arbitrate(context.Background(), appeals.ArbitrateRequest{
		AppealID:      a.ID,
		BranchID:      "b1",
		Decision:      "dismissed",
		DecisionNotes: "No merit found",
		ActorUserID:   "admin-1",
	})
	if err != nil {
		t.Fatalf("Arbitrate: %v", err)
	}
	if a.Status != "dismissed" {
		t.Errorf("expected dismissed, got %s", a.Status)
	}
	if arb.Decision != "dismissed" {
		t.Errorf("expected decision=dismissed, got %s", arb.Decision)
	}
}

// ── TestArbitrate_WrongState_ReturnsConflict ──────────────────────────────────

func TestArbitrate_WrongState_ReturnsConflict(t *testing.T) {
	repo := newStubRepo()
	svc := appeals.NewService(repo, nil)

	// Submit but don't review — can't arbitrate from submitted.
	a, _ := svc.Submit(context.Background(), appeals.SubmitRequest{
		BranchID: "b1", ReaderID: "r1", AppealType: "other", Reason: "test",
	})

	_, _, err := svc.Arbitrate(context.Background(), appeals.ArbitrateRequest{
		AppealID:      a.ID,
		BranchID:      "b1",
		Decision:      "upheld",
		DecisionNotes: "test",
		ActorUserID:   "admin-1",
	})
	if err == nil {
		t.Fatal("expected conflict error for arbitrating a non-under_review appeal")
	}
}

// ── TestReview_WrongState_ReturnsConflict ─────────────────────────────────────

func TestReview_WrongState_ReturnsConflict(t *testing.T) {
	repo := newStubRepo()
	svc := appeals.NewService(repo, nil)

	a, _ := svc.Submit(context.Background(), appeals.SubmitRequest{
		BranchID: "b1", ReaderID: "r1", AppealType: "other", Reason: "test",
	})
	// Move to under_review.
	a, _ = svc.Review(context.Background(), a.ID, "b1", "mod-1")
	// Try to review again.
	_, err := svc.Review(context.Background(), a.ID, "b1", "mod-2")
	if err == nil {
		t.Fatal("expected conflict error for reviewing an already-under_review appeal")
	}
}
