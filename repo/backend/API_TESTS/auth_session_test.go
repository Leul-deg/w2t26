package apitests

// auth_session_test.go covers the two auth endpoints that had no success-path tests:
//
//   GET  /api/v1/auth/me      (session hydration on page load)
//   POST /api/v1/auth/logout  (success path — 204 + cookie cleared)

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuth_Me_ReturnsCurrentUser verifies that GET /auth/me returns the
// authenticated user's profile with roles and permissions. This endpoint is
// called by the frontend on every page load to restore session state.
func TestAuth_Me_ReturnsCurrentUser(t *testing.T) {
	app := newTestApp(t)
	cookie := loginAs(t, app, "admin", "Admin1234!")

	rec := doRequest(t, app, http.MethodGet, "/api/v1/auth/me", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /auth/me with valid session should return 200: body=%s", rec.Body.String())

	var resp struct {
		User struct {
			Username string `json:"username"`
			ID       string `json:"id"`
		} `json:"user"`
		Roles       []string `json:"roles"`
		Permissions []string `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "admin", resp.User.Username,
		"me response must include the logged-in username")
	assert.NotEmpty(t, resp.User.ID, "user id must be present")
	assert.Contains(t, resp.Roles, "administrator",
		"admin user must have administrator role in me response")
	assert.NotEmpty(t, resp.Permissions,
		"admin user must have non-empty permissions list")
}

// TestAuth_Me_RequiresSession verifies that GET /auth/me returns 401 when no
// valid session cookie is present.
func TestAuth_Me_RequiresSession(t *testing.T) {
	app := newTestApp(t)

	rec := doRequest(t, app, http.MethodGet, "/api/v1/auth/me", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"GET /auth/me without session should return 401: body=%s", rec.Body.String())
}

// TestAuth_Logout_Success verifies the happy path for POST /auth/logout:
//   - Returns 204 No Content.
//   - Clears the lms_session cookie (MaxAge == -1).
//   - A subsequent GET /auth/me with the same cookie returns 401 (session invalidated).
func TestAuth_Logout_Success(t *testing.T) {
	app := newTestApp(t)
	cookie := loginAs(t, app, "admin", "Admin1234!")

	// Confirm we can hit a protected endpoint before logout.
	preRec := doRequest(t, app, http.MethodGet, "/api/v1/auth/me", nil, cookie)
	require.Equal(t, http.StatusOK, preRec.Code,
		"pre-logout GET /auth/me should return 200: body=%s", preRec.Body.String())

	// POST /auth/logout with the active session cookie.
	logoutRec := doRequest(t, app, http.MethodPost, "/api/v1/auth/logout", nil, cookie)
	assert.Equal(t, http.StatusNoContent, logoutRec.Code,
		"logout should return 204: body=%s", logoutRec.Body.String())

	// The response must set lms_session with MaxAge < 0 (clear the cookie).
	var clearedCookie bool
	for _, c := range logoutRec.Result().Cookies() {
		if c.Name == "lms_session" {
			clearedCookie = true
			assert.True(t, c.MaxAge < 0 || c.Value == "",
				"logout response must clear lms_session cookie (MaxAge < 0 or empty value)")
		}
	}
	assert.True(t, clearedCookie, "logout response must set lms_session cookie to clear it")

	// Subsequent GET /auth/me with the old cookie must return 401.
	postRec := doRequest(t, app, http.MethodGet, "/api/v1/auth/me", nil, cookie)
	assert.Equal(t, http.StatusUnauthorized, postRec.Code,
		"GET /auth/me after logout must return 401 (session invalidated): body=%s", postRec.Body.String())
}
