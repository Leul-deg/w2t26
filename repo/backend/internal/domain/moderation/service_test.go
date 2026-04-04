package moderation_test

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
	"lms/internal/domain/audit"
	"lms/internal/domain/moderation"
	"lms/internal/model"
)

// ── stub audit repository ─────────────────────────────────────────────────────

type captureAuditRepo struct {
	events []*model.AuditEvent
}

func (r *captureAuditRepo) Insert(_ context.Context, e *model.AuditEvent) error {
	r.events = append(r.events, e)
	return nil
}

func (r *captureAuditRepo) List(_ context.Context, _ audit.AuditFilter, _ model.Pagination) (model.PageResult[*model.AuditEvent], error) {
	return model.PageResult[*model.AuditEvent]{}, nil
}

// ── stub content updater ──────────────────────────────────────────────────────

type stubContentUpdater struct {
	items map[string]*model.GovernedContent
}

func (s *stubContentUpdater) GetByID(_ context.Context, id, _ string) (*model.GovernedContent, error) {
	if c, ok := s.items[id]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (s *stubContentUpdater) Update(_ context.Context, c *model.GovernedContent) error {
	s.items[c.ID] = c
	return nil
}

func (s *stubContentUpdater) UpdateStatus(_ context.Context, id, status string) error {
	if c, ok := s.items[id]; ok {
		c.Status = status
	}
	return nil
}

// ── stub moderation repository ────────────────────────────────────────────────

type stubModerationRepo struct {
	items  map[string]*model.ModerationItem
	nextID int
}

func newStubModerationRepo() *stubModerationRepo {
	return &stubModerationRepo{items: make(map[string]*model.ModerationItem)}
}

func (r *stubModerationRepo) Create(_ context.Context, item *model.ModerationItem) error {
	r.nextID++
	item.ID = fmt.Sprintf("item-%03d", r.nextID)
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()
	r.items[item.ID] = item
	return nil
}

func (r *stubModerationRepo) GetByID(_ context.Context, id, _ string) (*model.ModerationItem, error) {
	if item, ok := r.items[id]; ok {
		return item, nil
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (r *stubModerationRepo) GetByContentID(_ context.Context, contentID string) (*model.ModerationItem, error) {
	for _, item := range r.items {
		if item.ContentID == contentID && item.Status != "decided" {
			return item, nil
		}
	}
	return nil, fmt.Errorf("not found: %s", contentID)
}

func (r *stubModerationRepo) Decide(_ context.Context, id, decision, reason, decidedBy string) error {
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("not found: %s", id)
	}
	if decision == "" {
		s := decidedBy
		item.AssignedTo = &s
		item.Status = "in_review"
	} else {
		item.Status = "decided"
		item.Decision = strPtr(decision)
		item.DecisionReason = strPtr(reason)
		item.DecidedBy = strPtr(decidedBy)
		now := time.Now()
		item.DecidedAt = &now
	}
	return nil
}

func (r *stubModerationRepo) List(_ context.Context, _ string, _ moderation.ModerationFilter, _ model.Pagination) (model.PageResult[*model.ModerationItem], error) {
	items := make([]*model.ModerationItem, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return model.NewPageResult(items, len(items), model.Pagination{Page: 1, PerPage: 20}), nil
}

func strPtr(s string) *string { return &s }

// ── TestDecide_AuditEventRecorded ─────────────────────────────────────────────

func TestDecide_AuditEventRecorded(t *testing.T) {
	auditRepo := &captureAuditRepo{}
	logger := auditpkg.New(auditRepo)

	contentID := "content-001"
	content := &stubContentUpdater{
		items: map[string]*model.GovernedContent{
			contentID: {ID: contentID, Status: "pending_review", SubmittedBy: "user-author"},
		},
	}
	repo := newStubModerationRepo()

	item := &model.ModerationItem{ContentID: contentID, Status: "pending"}
	if err := repo.Create(context.Background(), item); err != nil {
		t.Fatalf("Create item: %v", err)
	}

	svc := moderation.NewService(repo, content, logger)

	result, err := svc.Decide(context.Background(), moderation.DecideRequest{
		ItemID:      item.ID,
		BranchID:    "",
		Decision:    "approved",
		Reason:      "content is appropriate",
		ActorUserID: "user-mod-001",
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if result.Status != "decided" {
		t.Errorf("expected status=decided, got %q", result.Status)
	}
	if len(auditRepo.events) == 0 {
		t.Fatal("expected at least one audit event, got none")
	}
	evt := auditRepo.events[0]
	if evt.EventType != model.AuditModerationDecision {
		t.Errorf("expected event_type=%q, got %q", model.AuditModerationDecision, evt.EventType)
	}
	if evt.ActorUserID == nil || *evt.ActorUserID != "user-mod-001" {
		t.Errorf("expected actor_user_id=user-mod-001, got %v", evt.ActorUserID)
	}
}

// ── TestDecide_AlreadyDecided_ReturnsConflict ─────────────────────────────────

func TestDecide_AlreadyDecided_ReturnsConflict(t *testing.T) {
	repo := newStubModerationRepo()
	content := &stubContentUpdater{items: map[string]*model.GovernedContent{
		"c1": {ID: "c1", Status: "approved"},
	}}
	svc := moderation.NewService(repo, content, nil)

	item := &model.ModerationItem{
		ContentID: "c1",
		Status:    "decided",
	}
	item.ID = "item-decided"
	repo.items[item.ID] = item

	_, err := svc.Decide(context.Background(), moderation.DecideRequest{
		ItemID:      "item-decided",
		BranchID:    "",
		Decision:    "approved",
		ActorUserID: "mod-1",
	})
	if err == nil {
		t.Fatal("expected error for already-decided item")
	}
}

// ── TestDecide_InvalidDecision_ReturnsValidation ──────────────────────────────

func TestDecide_InvalidDecision_ReturnsValidation(t *testing.T) {
	repo := newStubModerationRepo()
	content := &stubContentUpdater{items: map[string]*model.GovernedContent{}}
	svc := moderation.NewService(repo, content, nil)

	item := &model.ModerationItem{ContentID: "c1", Status: "pending"}
	_ = repo.Create(context.Background(), item)

	_, err := svc.Decide(context.Background(), moderation.DecideRequest{
		ItemID:      item.ID,
		BranchID:    "",
		Decision:    "maybe",
		ActorUserID: "mod-1",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid decision")
	}
}

// ── TestHandler_Decide_RequiresPermission ─────────────────────────────────────

func TestHandler_Decide_RequiresPermission(t *testing.T) {
	// nil service — permission check fires before any service call.
	h := moderation.NewHandler(nil)

	e := echo.New()
	e.POST("/moderation/items/:id/decide", h.Decide)

	// User without content:moderate permission.
	userWithoutPerm := &model.UserWithRoles{
		User:        &model.User{ID: "u-noperm"},
		Roles:       []string{},
		Permissions: []string{"content:read"},
	}

	req := httptest.NewRequest(http.MethodPost, "/moderation/items/item-1/decide",
		strings.NewReader(`{"decision":"approved"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("item-1")

	ctx := ctxutil.SetUser(c.Request().Context(), userWithoutPerm)
	c.SetRequest(c.Request().WithContext(ctx))

	err := h.Decide(c)
	if err == nil {
		t.Fatal("expected Forbidden error for missing content:moderate permission")
	}
}
