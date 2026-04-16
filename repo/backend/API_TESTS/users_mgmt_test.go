package apitests

// users_mgmt_test.go covers user management HTTP endpoints that were previously
// untested because test helpers bypassed the API via direct DB/repo calls:
//
//   POST   /api/v1/users                        (CreateUser)
//   POST   /api/v1/users/:id/roles              (AssignRole)
//   DELETE /api/v1/users/:id/roles/:role_id     (RevokeRole)
//   POST   /api/v1/users/:id/branches           (AssignBranch)
//   DELETE /api/v1/users/:id/branches/:branch_id (RevokeBranch)

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/tests/testdb"
)

// adminCookieForUsersTest returns a session cookie for the seeded admin user.
func adminCookieForUsersTest(t *testing.T, app *completeTestApp) *http.Cookie {
	t.Helper()
	return loginAs(t, app.testApp, "admin", "Admin1234!")
}

// ── POST /api/v1/users ────────────────────────────────────────────────────────

// TestUsers_CreateUser_Success verifies that admin can create a new user via HTTP
// and that the response contains the expected fields.
func TestUsers_CreateUser_Success(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookieForUsersTest(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "api-created-" + suffix

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/users", map[string]any{
		"username": username,
		"email":    username + "@test.local",
		"password": "ApiPassword123!",
	}, cookie)

	require.Equal(t, http.StatusCreated, rec.Code,
		"admin should create user: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, username, resp["username"], "response must contain correct username")
	assert.Contains(t, resp, "id", "response must contain id")
	assert.NotContains(t, rec.Body.String(), "password_hash", "password_hash must not be serialised")

	// Cleanup: deactivate the newly-created user.
	if id, ok := resp["id"].(string); ok {
		t.Cleanup(func() {
			pool := testdb.Open(t)
			defer pool.Close()
			pool.Exec(context.Background(), `UPDATE lms.users SET is_active = false WHERE id = $1`, id) //nolint
		})
	}
}

// TestUsers_CreateUser_RequiresPermission verifies that a non-admin user without
// users:write receives 403 on POST /users.
func TestUsers_CreateUser_RequiresPermission(t *testing.T) {
	app := newCompleteTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "no-userswrite-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/users", map[string]any{
		"username": "should-not-be-created-" + suffix,
		"email":    "blocked@test.local",
		"password": "Blocked123!",
	}, cookie)

	assert.Equal(t, http.StatusForbidden, rec.Code,
		"operations_staff without users:write should get 403: body=%s", rec.Body.String())
}

// TestUsers_CreateUser_MissingFields verifies that omitting required fields returns 422.
func TestUsers_CreateUser_MissingFields(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookieForUsersTest(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/users", map[string]any{
		"username": "",
		"email":    "",
		"password": "",
	}, cookie)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"missing required fields should return 422: body=%s", rec.Body.String())
}

// ── POST /api/v1/users/:id/roles ──────────────────────────────────────────────

// TestUsers_AssignRole_Success verifies that admin can assign a role to a user via
// HTTP and that a subsequent GET /users/:id reflects the assigned role.
func TestUsers_AssignRole_Success(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookieForUsersTest(t, app)

	// Create target user via the API (exercises POST /users as well).
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "role-target-" + suffix
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/users", map[string]any{
		"username": username,
		"email":    username + "@test.local",
		"password": "Target123!",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code, "create user: body=%s", createRec.Body.String())

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	targetID := createResp["id"].(string)
	t.Cleanup(func() {
		pool := testdb.Open(t)
		defer pool.Close()
		pool.Exec(context.Background(), `UPDATE lms.users SET is_active = false WHERE id = $1`, targetID) //nolint
		pool.Exec(context.Background(), `DELETE FROM lms.sessions WHERE user_id = $1`, targetID)          //nolint
	})

	// Look up the operations_staff role ID.
	pool := testdb.Open(t)
	defer pool.Close()
	var roleID string
	err := pool.QueryRow(context.Background(),
		`SELECT id::text FROM lms.roles WHERE name = 'operations_staff'`).Scan(&roleID)
	require.NoError(t, err, "find operations_staff role")

	// Assign the role via HTTP.
	assignRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/users/"+targetID+"/roles",
		map[string]any{"role_id": roleID},
		cookie,
	)
	require.Equal(t, http.StatusOK, assignRec.Code,
		"admin should assign role: body=%s", assignRec.Body.String())

	var assignResp map[string]any
	require.NoError(t, json.Unmarshal(assignRec.Body.Bytes(), &assignResp))
	assert.Equal(t, roleID, assignResp["role_id"], "response must echo back role_id")

	// Verify GET /users/:id includes the role.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/users/"+targetID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code)
	var getResp struct {
		Roles []struct {
			Name string `json:"name"`
		} `json:"roles"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	roleNames := make([]string, 0, len(getResp.Roles))
	for _, r := range getResp.Roles {
		roleNames = append(roleNames, r.Name)
	}
	assert.Contains(t, roleNames, "operations_staff",
		"GET /users/:id should show assigned role: got %v", roleNames)
}

// TestUsers_AssignRole_RequiresPermission verifies that a user without users:admin
// receives 403 on POST /users/:id/roles.
func TestUsers_AssignRole_RequiresPermission(t *testing.T) {
	app := newCompleteTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Caller has users:write but NOT users:admin.
	callerName := "write-no-admin-" + suffix
	callerID := createTestUser(t, app.testApp, callerName, callerName+"@test.local", "Password123!", "")
	roleID := createTempRoleWithPermissions(t, "users:write")
	assignUserRole(t, callerID, roleID)
	assignUserToBranch(t, callerID, mainBranchID)
	cookie := loginAs(t, app.testApp, callerName, "Password123!")

	// Target user on the same branch.
	targetName := "role-target2-" + suffix
	targetID := createTestUser(t, app.testApp, targetName, targetName+"@test.local", "Password123!", "")
	assignUserToBranch(t, targetID, mainBranchID)

	// Look up any role ID.
	pool := testdb.Open(t)
	defer pool.Close()
	var opsRoleID string
	err := pool.QueryRow(context.Background(),
		`SELECT id::text FROM lms.roles WHERE name = 'operations_staff'`).Scan(&opsRoleID)
	require.NoError(t, err)

	rec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/users/"+targetID+"/roles",
		map[string]any{"role_id": opsRoleID},
		cookie,
	)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"users:write without users:admin should get 403 on role assign: body=%s", rec.Body.String())
}

// ── DELETE /api/v1/users/:id/roles/:role_id ───────────────────────────────────

// TestUsers_RevokeRole_Success verifies that admin can remove a role from a user
// via HTTP and that a subsequent GET /users/:id no longer shows the role.
func TestUsers_RevokeRole_Success(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookieForUsersTest(t, app)

	// Create a target user.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "revoke-role-" + suffix
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/users", map[string]any{
		"username": username,
		"email":    username + "@test.local",
		"password": "Target123!",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	targetID := createResp["id"].(string)
	t.Cleanup(func() {
		pool := testdb.Open(t)
		defer pool.Close()
		pool.Exec(context.Background(), `UPDATE lms.users SET is_active = false WHERE id = $1`, targetID) //nolint
	})

	// Look up the operations_staff role ID.
	pool := testdb.Open(t)
	defer pool.Close()
	var opsRoleID string
	err := pool.QueryRow(context.Background(),
		`SELECT id::text FROM lms.roles WHERE name = 'operations_staff'`).Scan(&opsRoleID)
	require.NoError(t, err)

	// Assign the role via HTTP.
	assignRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/users/"+targetID+"/roles",
		map[string]any{"role_id": opsRoleID},
		cookie,
	)
	require.Equal(t, http.StatusOK, assignRec.Code)

	// Revoke the role via HTTP.
	revokeRec := doRequest(t, app.testApp, http.MethodDelete,
		"/api/v1/users/"+targetID+"/roles/"+opsRoleID,
		nil, cookie,
	)
	assert.Equal(t, http.StatusNoContent, revokeRec.Code,
		"admin should revoke role: body=%s", revokeRec.Body.String())

	// Confirm the role is gone via GET.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/users/"+targetID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code)
	var getResp struct {
		Roles []struct {
			Name string `json:"name"`
		} `json:"roles"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	roleNameList := make([]string, 0, len(getResp.Roles))
	for _, r := range getResp.Roles {
		roleNameList = append(roleNameList, r.Name)
	}
	assert.NotContains(t, roleNameList, "operations_staff",
		"revoked role must not appear in GET /users/:id: got %v", roleNameList)
}

// ── POST /api/v1/users/:id/branches ──────────────────────────────────────────

// TestUsers_AssignBranch_Success verifies that admin can assign a branch to a user
// via HTTP and that a subsequent GET /users/:id reflects the assignment.
func TestUsers_AssignBranch_Success(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookieForUsersTest(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "branch-assign-" + suffix
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/users", map[string]any{
		"username": username,
		"email":    username + "@test.local",
		"password": "Target123!",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	targetID := createResp["id"].(string)
	t.Cleanup(func() {
		pool := testdb.Open(t)
		defer pool.Close()
		pool.Exec(context.Background(), `UPDATE lms.users SET is_active = false WHERE id = $1`, targetID) //nolint
	})

	// Assign branch via HTTP.
	assignRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/users/"+targetID+"/branches",
		map[string]any{"branch_id": mainBranchID},
		cookie,
	)
	require.Equal(t, http.StatusOK, assignRec.Code,
		"admin should assign branch: body=%s", assignRec.Body.String())

	var assignResp map[string]any
	require.NoError(t, json.Unmarshal(assignRec.Body.Bytes(), &assignResp))
	assert.Equal(t, mainBranchID, assignResp["branch_id"],
		"response must echo back branch_id")

	// Confirm via GET /users/:id.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/users/"+targetID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code)
	var getResp struct {
		BranchIDs []string `json:"branch_ids"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	assert.Contains(t, getResp.BranchIDs, mainBranchID,
		"GET /users/:id should include assigned branch_id: got %v", getResp.BranchIDs)
}

// TestUsers_AssignBranch_RequiresPermission verifies that a user without users:admin
// receives 403 on POST /users/:id/branches.
func TestUsers_AssignBranch_RequiresPermission(t *testing.T) {
	app := newCompleteTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	callerName := "no-admin-branch-" + suffix
	callerID := createTestUser(t, app.testApp, callerName, callerName+"@test.local", "Password123!", "")
	assignUserRole(t, callerID, createTempRoleWithPermissions(t, "users:write"))
	assignUserToBranch(t, callerID, mainBranchID)
	cookie := loginAs(t, app.testApp, callerName, "Password123!")

	targetName := "branch-target2-" + suffix
	targetID := createTestUser(t, app.testApp, targetName, targetName+"@test.local", "Password123!", "")
	assignUserToBranch(t, targetID, mainBranchID)

	rec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/users/"+targetID+"/branches",
		map[string]any{"branch_id": mainBranchID},
		cookie,
	)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"users:write without users:admin should get 403 on branch assign: body=%s", rec.Body.String())
}

// ── DELETE /api/v1/users/:id/branches/:branch_id ─────────────────────────────

// TestUsers_RevokeBranch_Success verifies that admin can remove a branch assignment
// via HTTP and that a subsequent GET /users/:id no longer lists the branch.
func TestUsers_RevokeBranch_Success(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookieForUsersTest(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "revoke-branch-" + suffix
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/users", map[string]any{
		"username": username,
		"email":    username + "@test.local",
		"password": "Target123!",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	targetID := createResp["id"].(string)
	t.Cleanup(func() {
		pool := testdb.Open(t)
		defer pool.Close()
		pool.Exec(context.Background(), `UPDATE lms.users SET is_active = false WHERE id = $1`, targetID) //nolint
	})

	// Assign branch via HTTP.
	assignRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/users/"+targetID+"/branches",
		map[string]any{"branch_id": mainBranchID},
		cookie,
	)
	require.Equal(t, http.StatusOK, assignRec.Code)

	// Revoke the branch via HTTP.
	revokeRec := doRequest(t, app.testApp, http.MethodDelete,
		"/api/v1/users/"+targetID+"/branches/"+mainBranchID,
		nil, cookie,
	)
	assert.Equal(t, http.StatusNoContent, revokeRec.Code,
		"admin should revoke branch: body=%s", revokeRec.Body.String())

	// Confirm branch is gone via GET.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/users/"+targetID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code)
	var getResp struct {
		BranchIDs []string `json:"branch_ids"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	assert.NotContains(t, getResp.BranchIDs, mainBranchID,
		"revoked branch must not appear in GET /users/:id: got %v", getResp.BranchIDs)
}
