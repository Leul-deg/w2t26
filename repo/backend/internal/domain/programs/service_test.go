package programs_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"lms/internal/domain/programs"
	"lms/internal/model"
)

// ── stub repository ───────────────────────────────────────────────────────────

type stubProgramRepo struct {
	programs map[string]*model.Program
	nextID   int
}

func newStubProgramRepo() *stubProgramRepo {
	return &stubProgramRepo{programs: make(map[string]*model.Program)}
}

func (r *stubProgramRepo) Create(_ context.Context, p *model.Program) error {
	r.nextID++
	p.ID = fmt.Sprintf("prog-%03d", r.nextID)
	r.programs[p.ID] = p
	return nil
}

func (r *stubProgramRepo) GetByID(_ context.Context, id, _ string) (*model.Program, error) {
	if p, ok := r.programs[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (r *stubProgramRepo) Update(_ context.Context, p *model.Program) error {
	r.programs[p.ID] = p
	return nil
}

func (r *stubProgramRepo) UpdateStatus(_ context.Context, id, _, status string) error {
	if p, ok := r.programs[id]; ok {
		p.Status = status
	}
	return nil
}

func (r *stubProgramRepo) List(_ context.Context, _ string, _ programs.ProgramFilter, _ model.Pagination) (model.PageResult[*model.Program], error) {
	items := make([]*model.Program, 0, len(r.programs))
	for _, p := range r.programs {
		items = append(items, p)
	}
	return model.NewPageResult(items, len(items), model.Pagination{Page: 1, PerPage: 20}), nil
}

func (r *stubProgramRepo) GetPrerequisites(_ context.Context, _ string) ([]*model.ProgramPrerequisite, error) {
	return nil, nil
}

func (r *stubProgramRepo) AddPrerequisite(_ context.Context, pr *model.ProgramPrerequisite) error {
	return nil
}

func (r *stubProgramRepo) RemovePrerequisite(_ context.Context, _, _ string) error {
	return nil
}

func (r *stubProgramRepo) GetEnrollmentRules(_ context.Context, _ string) ([]*model.EnrollmentRule, error) {
	return nil, nil
}

func (r *stubProgramRepo) AddEnrollmentRule(_ context.Context, rule *model.EnrollmentRule) error {
	return nil
}

func (r *stubProgramRepo) RemoveEnrollmentRule(_ context.Context, _, _ string) error {
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func validCreateReq() programs.CreateRequest {
	now := time.Now()
	return programs.CreateRequest{
		BranchID:    "branch-1",
		Title:       "Story Time",
		Capacity:    20,
		StartsAt:    now.Add(time.Hour),
		EndsAt:      now.Add(2 * time.Hour),
		ActorUserID: "user-1",
	}
}

// ── Create validation ─────────────────────────────────────────────────────────

func TestPrograms_Create_MissingBranch(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	req := validCreateReq()
	req.BranchID = ""
	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error for missing branch_id")
	}
}

func TestPrograms_Create_MissingTitle(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	req := validCreateReq()
	req.Title = ""
	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error for missing title")
	}
}

func TestPrograms_Create_ZeroCapacity(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	req := validCreateReq()
	req.Capacity = 0
	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error for capacity < 1")
	}
}

func TestPrograms_Create_ZeroStartsAt(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	req := validCreateReq()
	req.StartsAt = time.Time{}
	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error for zero starts_at")
	}
}

func TestPrograms_Create_ZeroEndsAt(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	req := validCreateReq()
	req.EndsAt = time.Time{}
	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error for zero ends_at")
	}
}

func TestPrograms_Create_EndsBeforeStarts(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	req := validCreateReq()
	req.StartsAt = time.Now().Add(2 * time.Hour)
	req.EndsAt = time.Now().Add(time.Hour)
	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error when ends_at is before starts_at")
	}
}

func TestPrograms_Create_EnrollmentClosesBeforeOpens(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	req := validCreateReq()
	opens := time.Now().Add(30 * time.Minute)
	closes := time.Now().Add(10 * time.Minute) // before opens
	req.EnrollmentOpensAt = &opens
	req.EnrollmentClosesAt = &closes
	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error when enrollment_closes_at is before enrollment_opens_at")
	}
}

// TestPrograms_Create_Success verifies that a valid request creates a program
// with status "draft" and a default enrollment channel of "any".
func TestPrograms_Create_Success(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	req := validCreateReq()
	p, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != "draft" {
		t.Errorf("expected status=draft, got %q", p.Status)
	}
	if p.EnrollmentChannel != "any" {
		t.Errorf("expected enrollment_channel=any, got %q", p.EnrollmentChannel)
	}
}

// ── UpdateStatus validation ───────────────────────────────────────────────────

func TestPrograms_UpdateStatus_InvalidStatus(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	err := svc.UpdateStatus(context.Background(), "prog-1", "branch-1", "user-1", "bogus")
	if err == nil {
		t.Fatal("expected validation error for invalid status")
	}
}

func TestPrograms_UpdateStatus_ValidStatuses(t *testing.T) {
	for _, status := range []string{"draft", "published", "cancelled", "completed"} {
		repo := newStubProgramRepo()
		p := &model.Program{ID: "prog-1", Status: "draft"}
		repo.programs["prog-1"] = p

		svc := programs.NewService(repo, nil)
		err := svc.UpdateStatus(context.Background(), "prog-1", "branch-1", "user-1", status)
		if err != nil {
			t.Errorf("status %q should be valid, got error: %v", status, err)
		}
	}
}

// ── AddPrerequisite validation ────────────────────────────────────────────────

func TestPrograms_AddPrerequisite_SelfReference(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	_, err := svc.AddPrerequisite(context.Background(), "prog-1", "prog-1", nil)
	if err == nil {
		t.Fatal("expected validation error for self-prerequisite")
	}
}

func TestPrograms_AddPrerequisite_DifferentPrograms(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	pr, err := svc.AddPrerequisite(context.Background(), "prog-1", "prog-2", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.ProgramID != "prog-1" || pr.RequiredProgramID != "prog-2" {
		t.Errorf("prerequisite fields incorrect: %+v", pr)
	}
}

// ── AddEnrollmentRule validation ──────────────────────────────────────────────

func TestPrograms_AddEnrollmentRule_InvalidType(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	_, err := svc.AddEnrollmentRule(context.Background(), "prog-1", "neither", "field", "value", nil)
	if err == nil {
		t.Fatal("expected validation error for invalid rule_type")
	}
}

func TestPrograms_AddEnrollmentRule_EmptyMatchField(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	_, err := svc.AddEnrollmentRule(context.Background(), "prog-1", "whitelist", "", "value", nil)
	if err == nil {
		t.Fatal("expected validation error for empty match_field")
	}
}

func TestPrograms_AddEnrollmentRule_EmptyMatchValue(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	_, err := svc.AddEnrollmentRule(context.Background(), "prog-1", "blacklist", "status_code", "", nil)
	if err == nil {
		t.Fatal("expected validation error for empty match_value")
	}
}

func TestPrograms_AddEnrollmentRule_ValidRule(t *testing.T) {
	svc := programs.NewService(newStubProgramRepo(), nil)
	rule, err := svc.AddEnrollmentRule(context.Background(), "prog-1", "whitelist", "status_code", "active", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.RuleType != "whitelist" {
		t.Errorf("expected rule_type=whitelist, got %q", rule.RuleType)
	}
}

// ── Update validation ─────────────────────────────────────────────────────────

func TestPrograms_Update_MissingTitle(t *testing.T) {
	repo := newStubProgramRepo()
	p := &model.Program{ID: "prog-1", Status: "draft"}
	repo.programs["prog-1"] = p

	svc := programs.NewService(repo, nil)
	req := programs.UpdateRequest{
		Title:    "",
		Capacity: 10,
		StartsAt: time.Now().Add(time.Hour),
		EndsAt:   time.Now().Add(2 * time.Hour),
	}
	_, err := svc.Update(context.Background(), "prog-1", "branch-1", "user-1", req)
	if err == nil {
		t.Fatal("expected validation error for empty title")
	}
}
