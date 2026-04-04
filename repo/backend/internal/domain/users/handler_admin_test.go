package users_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/domain/users"
	"lms/internal/model"
)

func userWithPermissions(perms ...string) *model.UserWithRoles {
	return &model.UserWithRoles{
		User:        &model.User{ID: "u-test", Username: "tester"},
		Roles:       []string{},
		Permissions: perms,
	}
}

func newUserCtx(e *echo.Echo, method, path, body string) (echo.Context, *httptest.ResponseRecorder) {
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func withAdminUser(c echo.Context, u *model.UserWithRoles) echo.Context {
	ctx := ctxutil.SetUser(c.Request().Context(), u)
	ctx = ctxutil.SetBranchID(ctx, "")
	c.SetRequest(c.Request().WithContext(ctx))
	return c
}

func asForbiddenUser(err error, target **apperr.Forbidden) bool {
	if err == nil {
		return false
	}
	if f, ok := err.(*apperr.Forbidden); ok {
		*target = f
		return true
	}
	return false
}

// TestListUsers_RequiresUsersRead verifies that ListUsers rejects callers
// without users:read before any repo call is made.
func TestListUsers_RequiresUsersRead(t *testing.T) {
	e := echo.New()
	h := users.NewHandlerWithRepo(nil, nil)
	c, _ := newUserCtx(e, http.MethodGet, "/users", "")
	withAdminUser(c, userWithPermissions())

	err := h.ListUsers(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbiddenUser(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestCreateUser_RequiresUsersWrite verifies that CreateUser rejects callers
// without users:write before any repo call is made.
func TestCreateUser_RequiresUsersWrite(t *testing.T) {
	e := echo.New()
	h := users.NewHandlerWithRepo(nil, nil)
	c, _ := newUserCtx(e, http.MethodPost, "/users", `{"username":"x","email":"x@x.com","password":"pass"}`)
	withAdminUser(c, userWithPermissions("users:read"))

	err := h.CreateUser(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbiddenUser(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestGetUser_RequiresUsersRead verifies that GetUser rejects callers
// without users:read before any repo call is made.
func TestGetUser_RequiresUsersRead(t *testing.T) {
	e := echo.New()
	h := users.NewHandlerWithRepo(nil, nil)
	c, _ := newUserCtx(e, http.MethodGet, "/users/u1", "")
	withAdminUser(c, userWithPermissions())
	c.SetParamNames("id")
	c.SetParamValues("u1")

	err := h.GetUser(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbiddenUser(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestAssignRole_RequiresUsersAdmin verifies that AssignRole rejects callers
// without users:admin (users:read alone is insufficient).
func TestAssignRole_RequiresUsersAdmin(t *testing.T) {
	e := echo.New()
	h := users.NewHandlerWithRepo(nil, nil)
	c, _ := newUserCtx(e, http.MethodPost, "/users/u1/roles", `{"role_id":"r1"}`)
	withAdminUser(c, userWithPermissions("users:read", "users:write"))
	c.SetParamNames("id")
	c.SetParamValues("u1")

	err := h.AssignRole(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbiddenUser(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}

// TestAssignBranch_RequiresUsersAdmin verifies that AssignBranch rejects callers
// without users:admin (users:read alone is insufficient).
func TestAssignBranch_RequiresUsersAdmin(t *testing.T) {
	e := echo.New()
	h := users.NewHandlerWithRepo(nil, nil)
	c, _ := newUserCtx(e, http.MethodPost, "/users/u1/branches", `{"branch_id":"b1"}`)
	withAdminUser(c, userWithPermissions("users:read", "users:write"))
	c.SetParamNames("id")
	c.SetParamValues("u1")

	err := h.AssignBranch(c)
	if err == nil {
		t.Fatal("expected Forbidden, got nil")
	}
	var fe *apperr.Forbidden
	if !asForbiddenUser(err, &fe) {
		t.Fatalf("expected *apperr.Forbidden, got %T", err)
	}
}
