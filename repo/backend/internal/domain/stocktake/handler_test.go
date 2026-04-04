package stocktake_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/domain/stocktake"
	"lms/internal/model"
)

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

func userWithPerms(perms ...string) *model.UserWithRoles {
	return &model.UserWithRoles{
		User:        &model.User{ID: "u-test"},
		Permissions: perms,
	}
}

func withUser(c echo.Context, user *model.UserWithRoles) echo.Context {
	ctx := ctxutil.SetUser(c.Request().Context(), user)
	ctx = ctxutil.SetBranchID(ctx, "branch-1")
	c.SetRequest(c.Request().WithContext(ctx))
	return c
}

// TestListSessions_RequiresStocktakeRead verifies GET /stocktake returns Forbidden
// when caller lacks stocktake:read.
func TestListSessions_RequiresStocktakeRead(t *testing.T) {
	e := echo.New()
	h := stocktake.NewHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/stocktake", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	withUser(c, userWithPerms("holdings:read")) // no stocktake:read

	err := h.ListSessions(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestCreateSession_RequiresStocktakeWrite verifies POST /stocktake returns Forbidden
// when caller lacks stocktake:write.
func TestCreateSession_RequiresStocktakeWrite(t *testing.T) {
	e := echo.New()
	h := stocktake.NewHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/stocktake",
		strings.NewReader(`{"name":"Annual Count"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	withUser(c, userWithPerms("stocktake:read")) // read but not write

	err := h.CreateSession(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestRecordScan_RequiresStocktakeWrite verifies POST /stocktake/:id/scan returns
// Forbidden when caller lacks stocktake:write.
func TestRecordScan_RequiresStocktakeWrite(t *testing.T) {
	e := echo.New()
	h := stocktake.NewHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/stocktake/s-001/scan",
		strings.NewReader(`{"barcode":"ABC123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("s-001")
	withUser(c, userWithPerms("stocktake:read")) // read only

	err := h.RecordScan(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestCloseSession_RequiresStocktakeWrite verifies PATCH /stocktake/:id/status returns
// Forbidden when caller lacks stocktake:write.
func TestCloseSession_RequiresStocktakeWrite(t *testing.T) {
	e := echo.New()
	h := stocktake.NewHandler(nil)

	req := httptest.NewRequest(http.MethodPatch, "/stocktake/s-001/status",
		strings.NewReader(`{"status":"closed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("s-001")
	withUser(c, userWithPerms("stocktake:read")) // read only

	err := h.CloseSession(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestGetVariances_RequiresStocktakeRead verifies GET /stocktake/:id/variances returns
// Forbidden when caller lacks stocktake:read.
func TestGetVariances_RequiresStocktakeRead(t *testing.T) {
	e := echo.New()
	h := stocktake.NewHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/stocktake/s-001/variances", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("s-001")
	withUser(c, userWithPerms("holdings:read")) // no stocktake:read

	err := h.GetVariances(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}
