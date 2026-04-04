package holdings_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/domain/holdings"
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

func newCtx(e *echo.Echo, method, path string, body string) (echo.Context, *httptest.ResponseRecorder) {
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

func withUser(c echo.Context, user *model.UserWithRoles) echo.Context {
	ctx := ctxutil.SetUser(c.Request().Context(), user)
	ctx = ctxutil.SetBranchID(ctx, "branch-1")
	c.SetRequest(c.Request().WithContext(ctx))
	return c
}

// TestListHoldings_RequiresHoldingsRead verifies GET /holdings returns Forbidden
// when caller lacks holdings:read.
func TestListHoldings_RequiresHoldingsRead(t *testing.T) {
	e := echo.New()
	h := holdings.NewHandler(nil)
	c, _ := newCtx(e, http.MethodGet, "/holdings", "")
	withUser(c, userWithPerms("readers:read"))

	err := h.ListHoldings(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestCreateHolding_RequiresHoldingsWrite verifies POST /holdings returns Forbidden
// when caller lacks holdings:write.
func TestCreateHolding_RequiresHoldingsWrite(t *testing.T) {
	e := echo.New()
	h := holdings.NewHandler(nil)
	c, _ := newCtx(e, http.MethodPost, "/holdings", `{"title":"Test","language":"en"}`)
	withUser(c, userWithPerms("holdings:read")) // read but not write

	err := h.CreateHolding(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestAddCopy_RequiresCopiesWrite verifies POST /holdings/:id/copies returns Forbidden
// when caller lacks copies:write.
func TestAddCopy_RequiresCopiesWrite(t *testing.T) {
	e := echo.New()
	h := holdings.NewHandler(nil)
	c, _ := newCtx(e, http.MethodPost, "/holdings/h-001/copies", `{"barcode":"ABC123","status_code":"available","condition":"good"}`)
	c.SetParamNames("id")
	c.SetParamValues("h-001")
	withUser(c, userWithPerms("holdings:read", "copies:read")) // no copies:write

	err := h.AddCopy(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestListCopies_RequiresCopiesRead verifies GET /holdings/:id/copies returns Forbidden
// when caller lacks copies:read.
func TestListCopies_RequiresCopiesRead(t *testing.T) {
	e := echo.New()
	h := holdings.NewHandler(nil)
	c, _ := newCtx(e, http.MethodGet, "/holdings/h-001/copies", "")
	c.SetParamNames("id")
	c.SetParamValues("h-001")
	withUser(c, userWithPerms("holdings:read")) // no copies:read

	err := h.ListCopies(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestDeactivateHolding_RequiresHoldingsWrite verifies DELETE /holdings/:id returns
// Forbidden when caller lacks holdings:write.
func TestDeactivateHolding_RequiresHoldingsWrite(t *testing.T) {
	e := echo.New()
	h := holdings.NewHandler(nil)
	c, _ := newCtx(e, http.MethodDelete, "/holdings/h-001", "")
	c.SetParamNames("id")
	c.SetParamValues("h-001")
	withUser(c, userWithPerms("holdings:read")) // read only

	err := h.DeactivateHolding(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbidden(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}
