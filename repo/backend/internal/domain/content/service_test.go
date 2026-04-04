package content_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"lms/internal/ctxutil"
	"lms/internal/domain/content"
	"lms/internal/model"
)

// ── stub repository ───────────────────────────────────────────────────────────

type stubContentRepo struct {
	items  map[string]*model.GovernedContent
	nextID int
}

func newStubRepo() *stubContentRepo {
	return &stubContentRepo{items: make(map[string]*model.GovernedContent)}
}

func (r *stubContentRepo) Create(_ context.Context, c *model.GovernedContent) error {
	r.nextID++
	c.ID = fmt.Sprintf("content-%03d", r.nextID)
	c.SubmittedAt = time.Now()
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	r.items[c.ID] = c
	return nil
}

func (r *stubContentRepo) GetByID(_ context.Context, id, _ string) (*model.GovernedContent, error) {
	if c, ok := r.items[id]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (r *stubContentRepo) Update(_ context.Context, c *model.GovernedContent) error {
	r.items[c.ID] = c
	return nil
}

func (r *stubContentRepo) UpdateStatus(_ context.Context, id, status string) error {
	if c, ok := r.items[id]; ok {
		c.Status = status
		return nil
	}
	return fmt.Errorf("not found: %s", id)
}

func (r *stubContentRepo) List(_ context.Context, _ string, _ content.ContentFilter, _ model.Pagination) (model.PageResult[*model.GovernedContent], error) {
	items := make([]*model.GovernedContent, 0, len(r.items))
	for _, c := range r.items {
		items = append(items, c)
	}
	return model.NewPageResult(items, len(items), model.Pagination{Page: 1, PerPage: 20}), nil
}

type stubModerationQueue struct {
	contentIDs []string
	fail       error
}

func (s *stubModerationQueue) CreateItemForContent(_ context.Context, contentID string) (*model.ModerationItem, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	s.contentIDs = append(s.contentIDs, contentID)
	return &model.ModerationItem{ID: "mod-001", ContentID: contentID, Status: "pending"}, nil
}

// ── TestCreate_HappyPath ──────────────────────────────────────────────────────

func TestCreate_HappyPath(t *testing.T) {
	svc := content.NewService(newStubRepo(), nil, nil)

	item, err := svc.Create(context.Background(), content.CreateRequest{
		BranchID:    "branch-001",
		Title:       "Summer Reading Guide",
		ContentType: "announcement",
		ActorUserID: "user-001",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if item.Status != "draft" {
		t.Errorf("expected status=draft, got %q", item.Status)
	}
	if item.ID == "" {
		t.Error("expected non-empty ID")
	}
}

// ── TestCreate_InvalidContentType ────────────────────────────────────────────

func TestCreate_InvalidContentType(t *testing.T) {
	svc := content.NewService(newStubRepo(), nil, nil)
	_, err := svc.Create(context.Background(), content.CreateRequest{
		BranchID:    "b1",
		Title:       "Test",
		ContentType: "blogpost", // invalid
		ActorUserID: "u1",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid content_type")
	}
}

// ── TestSubmit_TransitionsDraftToPendingReview ────────────────────────────────

func TestSubmit_TransitionsDraftToPendingReview(t *testing.T) {
	repo := newStubRepo()
	queue := &stubModerationQueue{}
	svc := content.NewService(repo, queue, nil)

	item, _ := svc.Create(context.Background(), content.CreateRequest{
		BranchID:    "b1",
		Title:       "Test",
		ContentType: "announcement",
		ActorUserID: "user-001",
	})

	submitted, err := svc.Submit(context.Background(), item.ID, "b1", "user-001")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submitted.Status != "pending_review" {
		t.Errorf("expected pending_review, got %q", submitted.Status)
	}
	if len(queue.contentIDs) != 1 || queue.contentIDs[0] != item.ID {
		t.Fatalf("expected moderation queue item to be created for %s, got %#v", item.ID, queue.contentIDs)
	}
}

// ── TestSubmit_WrongAuthor_ReturnsForbidden ───────────────────────────────────

func TestSubmit_WrongAuthor_ReturnsForbidden(t *testing.T) {
	repo := newStubRepo()
	svc := content.NewService(repo, nil, nil)

	item, err := svc.Create(context.Background(), content.CreateRequest{
		BranchID: "b1", Title: "T", ContentType: "announcement", ActorUserID: "author-1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, submitErr := svc.Submit(context.Background(), item.ID, "b1", "different-user")
	if submitErr == nil {
		t.Fatal("expected Forbidden error when non-author submits")
	}
}

// ── TestUpdate_DraftOnlyEditable ─────────────────────────────────────────────

func TestUpdate_DraftOnlyEditable(t *testing.T) {
	repo := newStubRepo()
	svc := content.NewService(repo, nil, nil)

	docItem, docErr := svc.Create(context.Background(), content.CreateRequest{
		BranchID: "b1", Title: "T", ContentType: "document", ActorUserID: "u1",
	})
	if docErr != nil {
		t.Fatalf("Create: %v", docErr)
	}
	// Submit to move out of draft.
	_, _ = svc.Submit(context.Background(), docItem.ID, "b1", "u1")

	newTitle := "Updated Title"
	_, err := svc.Update(context.Background(), content.UpdateRequest{
		ID: docItem.ID, BranchID: "b1", Title: &newTitle, ActorUserID: "u1",
	})
	if err == nil {
		t.Fatal("expected Conflict error when editing non-draft item")
	}
}

// ── TestHandler_Create_RequiresPermission ─────────────────────────────────────

func TestHandler_Create_RequiresPermission(t *testing.T) {
	h := content.NewHandler(nil)

	e := echo.New()
	e.POST("/content", h.Create)

	userWithoutPerm := &model.UserWithRoles{
		User:        &model.User{ID: "u-noperm"},
		Permissions: []string{"content:read"}, // missing content:submit
	}

	req := httptest.NewRequest(http.MethodPost, "/content",
		strings.NewReader(`{"title":"T","content_type":"announcement"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)
	ctx := ctxutil.SetUser(c.Request().Context(), userWithoutPerm)
	c.SetRequest(c.Request().WithContext(ctx))

	err := h.Create(c)
	if err == nil {
		t.Fatal("expected Forbidden error for missing content:submit permission")
	}
}

// ── TestHandler_Publish_RequiresPermission ────────────────────────────────────

func TestHandler_Publish_RequiresPermission(t *testing.T) {
	h := content.NewHandler(nil)

	e := echo.New()
	e.POST("/content/:id/publish", h.Publish)

	userWithoutPerm := &model.UserWithRoles{
		User:        &model.User{ID: "u-noperm"},
		Permissions: []string{"content:read", "content:submit"}, // missing content:publish
	}

	req := httptest.NewRequest(http.MethodPost, "/content/c1/publish", nil)
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("c1")
	ctx := ctxutil.SetUser(c.Request().Context(), userWithoutPerm)
	c.SetRequest(c.Request().WithContext(ctx))

	err := h.Publish(c)
	if err == nil {
		t.Fatal("expected Forbidden error for missing content:publish permission")
	}
}
