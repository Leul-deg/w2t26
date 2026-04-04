package circulation_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/domain/circulation"
	"lms/internal/model"
)

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

func withUser(c echo.Context, perms ...string) echo.Context {
	user := &model.UserWithRoles{User: &model.User{ID: "u-test"}, Permissions: perms}
	ctx := ctxutil.SetUser(c.Request().Context(), user)
	ctx = ctxutil.SetBranchID(ctx, "branch-1")
	c.SetRequest(c.Request().WithContext(ctx))
	return c
}

// TestCheckout_RequiresCirculationWrite confirms POST /checkout returns Forbidden
// when the caller lacks circulation:write.
func TestCheckout_RequiresCirculationWrite(t *testing.T) {
	e := echo.New()
	h := circulation.NewHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/circulation/checkout",
		strings.NewReader(`{"barcode":"ABC","reader_id":"r1","due_date":"2026-05-01"}`))
	req.Header.Set("Content-Type", "application/json")
	c := e.NewContext(req, httptest.NewRecorder())
	withUser(c, "circulation:read") // no write

	err := h.Checkout(c)
	if err == nil {
		t.Fatal("expected Forbidden")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestReturn_RequiresCirculationWrite confirms POST /return returns Forbidden
// when the caller lacks circulation:write.
func TestReturn_RequiresCirculationWrite(t *testing.T) {
	e := echo.New()
	h := circulation.NewHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/circulation/return",
		strings.NewReader(`{"barcode":"ABC"}`))
	req.Header.Set("Content-Type", "application/json")
	c := e.NewContext(req, httptest.NewRecorder())
	withUser(c, "circulation:read")

	err := h.Return(c)
	if err == nil {
		t.Fatal("expected Forbidden")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestListByBranch_RequiresCirculationRead confirms GET /circulation returns
// Forbidden when the caller has no circulation permission.
func TestListByBranch_RequiresCirculationRead(t *testing.T) {
	e := echo.New()
	h := circulation.NewHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/circulation", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	withUser(c, "holdings:read") // unrelated permission

	err := h.ListByBranch(c)
	if err == nil {
		t.Fatal("expected Forbidden")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}
