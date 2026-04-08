package exports_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"lms/internal/apierr"
	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/domain/exports"
	"lms/internal/model"
)

// stubExportRepo satisfies exports.Repository without a real database.
type stubExportRepo struct{}

func (r *stubExportRepo) Create(_ context.Context, _ *model.ExportJob) error          { return nil }
func (r *stubExportRepo) Finalise(_ context.Context, _ string, _ int, _ string) error { return nil }
func (r *stubExportRepo) List(_ context.Context, _ string, p model.Pagination) (model.PageResult[*model.ExportJob], error) {
	return model.NewPageResult([]*model.ExportJob{}, 0, p), nil
}

// TestExportReaders_MissingPermission verifies that POST /exports/readers returns
// a Forbidden error when the caller does not hold the exports:create permission.
func TestExportReaders_MissingPermission(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = apierr.ErrorHandler

	userWithoutPerm := &model.UserWithRoles{
		User:        &model.User{ID: "u-001"},
		Roles:       []string{"content_moderator"},
		Permissions: []string{"content:read"}, // deliberately no exports:create
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/exports/readers", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	ctx := ctxutil.SetUser(req.Context(), userWithoutPerm)
	ctx = ctxutil.SetBranchID(ctx, "branch-1")
	c.SetRequest(req.WithContext(ctx))

	// Service is nil — the handler must check permissions before calling it.
	h := exports.NewHandler(nil)
	err := h.ExportReaders(c)

	if err == nil {
		t.Fatal("expected a Forbidden error, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Errorf("expected *apperr.Forbidden, got %T: %v", err, err)
		return
	}
	if fe.Resource != "readers" {
		t.Errorf("expected resource=readers, got %q", fe.Resource)
	}
	if fe.Action != "export" {
		t.Errorf("expected action=export, got %q", fe.Action)
	}
}

// TestExportHoldings_MissingPermission mirrors the above for holdings.
func TestExportHoldings_MissingPermission(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = apierr.ErrorHandler

	userWithoutPerm := &model.UserWithRoles{
		User:        &model.User{ID: "u-002"},
		Roles:       []string{"operations_staff"},
		Permissions: []string{"readers:read"}, // no exports:create
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/exports/holdings", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	ctx := ctxutil.SetUser(req.Context(), userWithoutPerm)
	ctx = ctxutil.SetBranchID(ctx, "branch-2")
	c.SetRequest(req.WithContext(ctx))

	h := exports.NewHandler(nil)
	err := h.ExportHoldings(c)

	if err == nil {
		t.Fatal("expected a Forbidden error for holdings export without permission")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Errorf("expected *apperr.Forbidden, got %T: %v", err, err)
	}
}

// TestListExportJobs_MissingPermission verifies that GET /exports returns 403
// when the caller does not hold exports:create.
func TestListExportJobs_MissingPermission(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = apierr.ErrorHandler

	userWithoutPerm := &model.UserWithRoles{
		User:        &model.User{ID: "u-003"},
		Roles:       []string{"reader"},
		Permissions: []string{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/exports", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	ctx := ctxutil.SetUser(req.Context(), userWithoutPerm)
	ctx = ctxutil.SetBranchID(ctx, "branch-1")
	c.SetRequest(req.WithContext(ctx))

	h := exports.NewHandler(nil)
	err := h.ListJobs(c)

	if err == nil {
		t.Fatal("expected Forbidden error listing export jobs without permission")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Errorf("expected *apperr.Forbidden, got %T: %v", err, err)
	}
}

// asForbidden unwraps errors looking for *apperr.Forbidden.
func asForbidden(err error, target **apperr.Forbidden) bool {
	if err == nil {
		return false
	}
	if f, ok := err.(*apperr.Forbidden); ok {
		*target = f
		return true
	}
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return asForbidden(u.Unwrap(), target)
	}
	return false
}
