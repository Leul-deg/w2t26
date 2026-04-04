package feedback_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	auditpkg "lms/internal/audit"
	"lms/internal/ctxutil"
	domainaudit "lms/internal/domain/audit"
	"lms/internal/domain/feedback"
	"lms/internal/model"
)

// ── stub feedback repository ──────────────────────────────────────────────────

type stubFeedbackRepo struct {
	items  map[string]*model.Feedback
	tags   []*model.FeedbackTag
	nextID int
}

func newStubFeedbackRepo() *stubFeedbackRepo {
	return &stubFeedbackRepo{
		items: make(map[string]*model.Feedback),
		tags: []*model.FeedbackTag{
			{ID: "tag-001", Name: "helpful", IsActive: true},
			{ID: "tag-002", Name: "outdated", IsActive: true},
		},
	}
}

func (r *stubFeedbackRepo) Create(_ context.Context, f *model.Feedback, tagIDs []string) error {
	r.nextID++
	f.ID = fmt.Sprintf("fb-%03d", r.nextID)
	f.SubmittedAt = time.Now()
	r.items[f.ID] = f
	return nil
}

func (r *stubFeedbackRepo) GetByID(_ context.Context, id, _ string) (*model.Feedback, error) {
	if f, ok := r.items[id]; ok {
		return f, nil
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (r *stubFeedbackRepo) Moderate(_ context.Context, id, status, moderatedBy string) error {
	if f, ok := r.items[id]; ok {
		f.Status = status
		f.ModeratedBy = strPtr(moderatedBy)
		return nil
	}
	return fmt.Errorf("not found: %s", id)
}

func (r *stubFeedbackRepo) List(_ context.Context, _ string, _ feedback.FeedbackFilter, _ model.Pagination) (model.PageResult[*model.Feedback], error) {
	items := make([]*model.Feedback, 0, len(r.items))
	for _, f := range r.items {
		items = append(items, f)
	}
	return model.NewPageResult(items, len(items), model.Pagination{Page: 1, PerPage: 20}), nil
}

func (r *stubFeedbackRepo) ListTags(_ context.Context) ([]*model.FeedbackTag, error) {
	return r.tags, nil
}

func strPtr(s string) *string { return &s }

type captureAuditRepo struct {
	events []*model.AuditEvent
}

func (r *captureAuditRepo) Insert(_ context.Context, e *model.AuditEvent) error {
	r.events = append(r.events, e)
	return nil
}

func (r *captureAuditRepo) List(_ context.Context, _ domainaudit.AuditFilter, _ model.Pagination) (model.PageResult[*model.AuditEvent], error) {
	return model.PageResult[*model.AuditEvent]{}, nil
}

// ── TestSubmit_HappyPath ──────────────────────────────────────────────────────

func TestSubmit_HappyPath(t *testing.T) {
	svc := feedback.NewService(newStubFeedbackRepo(), nil)
	rating := 4

	f, err := svc.Submit(context.Background(), feedback.SubmitRequest{
		BranchID:    "branch-001",
		ReaderID:    "reader-001",
		TargetType:  "program",
		TargetID:    "prog-001",
		Rating:      &rating,
		TagNames:    []string{"helpful"},
		ActorUserID: "staff-001",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if f.Status != "pending" {
		t.Errorf("expected status=pending, got %q", f.Status)
	}
	if f.ID == "" {
		t.Error("expected non-empty ID")
	}
	if *f.Rating != 4 {
		t.Errorf("expected rating=4, got %d", *f.Rating)
	}
}

// ── TestSubmit_UnknownTag_ReturnsValidation ───────────────────────────────────

func TestSubmit_UnknownTag_ReturnsValidation(t *testing.T) {
	svc := feedback.NewService(newStubFeedbackRepo(), nil)
	rating := 3

	_, err := svc.Submit(context.Background(), feedback.SubmitRequest{
		BranchID:    "branch-001",
		ReaderID:    "reader-001",
		TargetType:  "program",
		TargetID:    "prog-001",
		Rating:      &rating,
		TagNames:    []string{"nonexistent_tag"},
		ActorUserID: "staff-001",
	})
	if err == nil {
		t.Fatal("expected validation error for unknown tag")
	}
}

// ── TestSubmit_InvalidRating_ReturnsValidation ────────────────────────────────

func TestSubmit_InvalidRating_ReturnsValidation(t *testing.T) {
	svc := feedback.NewService(newStubFeedbackRepo(), nil)
	rating := 6 // out of range

	_, err := svc.Submit(context.Background(), feedback.SubmitRequest{
		BranchID:    "branch-001",
		ReaderID:    "reader-001",
		TargetType:  "program",
		TargetID:    "prog-001",
		Rating:      &rating,
		ActorUserID: "staff-001",
	})
	if err == nil {
		t.Fatal("expected validation error for rating > 5")
	}
}

// ── TestSubmit_MissingRatingAndComment_ReturnsValidation ─────────────────────

func TestSubmit_MissingRatingAndComment_ReturnsValidation(t *testing.T) {
	svc := feedback.NewService(newStubFeedbackRepo(), nil)

	_, err := svc.Submit(context.Background(), feedback.SubmitRequest{
		BranchID:    "branch-001",
		ReaderID:    "reader-001",
		TargetType:  "program",
		TargetID:    "prog-001",
		ActorUserID: "staff-001",
		// No rating, no comment.
	})
	if err == nil {
		t.Fatal("expected validation error when neither rating nor comment provided")
	}
}

// ── TestModerate_HappyPath ────────────────────────────────────────────────────

func TestModerate_HappyPath(t *testing.T) {
	repo := newStubFeedbackRepo()
	auditRepo := &captureAuditRepo{}
	logger := auditpkg.New(auditRepo)
	svc := feedback.NewService(repo, logger)
	rating := 2

	f, _ := svc.Submit(context.Background(), feedback.SubmitRequest{
		BranchID:    "b1",
		ReaderID:    "r1",
		TargetType:  "holding",
		TargetID:    "h1",
		Rating:      &rating,
		ActorUserID: "staff-1",
	})

	updated, err := svc.Moderate(context.Background(), feedback.ModerateRequest{
		ID:          f.ID,
		BranchID:    "b1",
		Status:      "approved",
		ActorUserID: "mod-1",
	})
	if err != nil {
		t.Fatalf("Moderate: %v", err)
	}
	if updated.Status != "approved" {
		t.Errorf("expected status=approved, got %q", updated.Status)
	}
	if len(auditRepo.events) == 0 {
		t.Fatal("expected a feedback moderation audit event")
	}
	evt := auditRepo.events[len(auditRepo.events)-1]
	if evt.EventType != model.AuditFeedbackModerated {
		t.Fatalf("expected %q, got %q", model.AuditFeedbackModerated, evt.EventType)
	}
}

// ── TestHandler_Moderate_RequiresPermission ───────────────────────────────────

func TestHandler_Moderate_RequiresPermission(t *testing.T) {
	h := feedback.NewHandler(nil)

	e := echo.New()
	e.POST("/feedback/:id/moderate", h.Moderate)

	userWithoutPerm := &model.UserWithRoles{
		User:        &model.User{ID: "u-noperm"},
		Permissions: []string{"feedback:read"}, // missing feedback:moderate
	}

	req := httptest.NewRequest(http.MethodPost, "/feedback/fb-001/moderate",
		strings.NewReader(`{"status":"approved"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("fb-001")

	ctx := ctxutil.SetUser(c.Request().Context(), userWithoutPerm)
	c.SetRequest(c.Request().WithContext(ctx))

	err := h.Moderate(c)
	if err == nil {
		t.Fatal("expected Forbidden error for missing feedback:moderate permission")
	}
}
