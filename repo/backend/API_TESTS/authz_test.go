package apitests

// authz_test.go covers object-level and branch-scope authorization invariants
// that the domain-permission tests (domain_perms_test.go) do not reach:
//
//   - Circulation copy_id cross-branch IDOR prevention
//   - GET /circulation/active/:id visibility across branches
//   - Program rule endpoints branch-authorize via parent program lookup
//   - Unassigned-user sentinel UUID path (no branch → empty results, not global scope)
//   - Admin (empty branchID) list behavior for circulation, programs, and users

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/internal/apierr"
	auditpkg "lms/internal/audit"
	"lms/internal/domain/circulation"
	copies "lms/internal/domain/copies"
	"lms/internal/domain/programs"
	"lms/internal/domain/readers"
	"lms/internal/domain/reports"
	"lms/internal/domain/users"
	appmw "lms/internal/middleware"
	"lms/internal/store/postgres"
	"lms/tests/testdb"
)

// authzTestApp wraps testApp and exposes the underlying pool for direct SQL inserts.
type authzTestApp struct {
	*testApp
}

// newAuthzTestApp builds an Echo application with auth, readers, circulation,
// programs, and users routes registered. It is used by authz tests that need
// the full routing surface for cross-domain authorization scenarios.
func newAuthzTestApp(t *testing.T) *authzTestApp {
	t.Helper()
	pool := testdb.Open(t)
	t.Cleanup(func() { pool.Close() })

	userRepo := postgres.NewUserRepo(pool)
	sessionRepo := postgres.NewSessionRepo(pool)
	captchaRepo := postgres.NewCaptchaRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)
	auditLogger := auditpkg.New(auditRepo)

	authService := users.NewService(userRepo, sessionRepo, captchaRepo, auditLogger, 1800)

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = apierr.ErrorHandler

	requireAuth := appmw.RequireAuth(sessionRepo, userRepo, 1800)
	branchScopeMW := appmw.BranchScope(userRepo)

	api := e.Group("/api/v1")

	// Auth + users management routes.
	authHandler := users.NewHandlerWithRepo(authService, userRepo, auditLogger)
	authHandler.RegisterRoutes(api, requireAuth)
	authHandler.RegisterUserRoutes(api, requireAuth, branchScopeMW)

	// Readers routes (needed for sentinel test).
	readerRepo := postgres.NewReaderRepo(pool)
	readerService := readers.NewService(readerRepo, nil, auditLogger)
	readersHandler := readers.NewHandler(readerService, auditLogger)
	readersHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// Circulation routes.
	circulationRepo := postgres.NewCirculationRepo(pool)
	copyRepo := postgres.NewCopyRepo(pool)
	circulationService := circulation.NewService(circulationRepo, copies.Repository(copyRepo), auditLogger)
	circulationHandler := circulation.NewHandler(circulationService)
	circulationHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// Programs routes.
	programRepo := postgres.NewProgramRepo(pool)
	programService := programs.NewService(programRepo, auditLogger)
	programsHandler := programs.NewHandler(programService)
	programsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// Reports routes.
	exportRepo := postgres.NewExportRepo(pool)
	reportsRepo := postgres.NewReportsRepo(pool)
	reportsService := reports.NewService(reportsRepo, exportRepo, auditLogger)
	reportsHandler := reports.NewHandler(reportsService)
	reportsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	base := &testApp{
		e:           e,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		captchaRepo: captchaRepo,
		auditRepo:   auditRepo,
		authService: authService,
		auditLogger: auditLogger,
	}
	return &authzTestApp{testApp: base}
}

// assignUserToBranch inserts a branch assignment for a user. The pool is opened
// and closed within this helper to avoid reusing a pool that the test has already
// scheduled for cleanup.
func assignUserToBranch(t *testing.T, userID, branchID string) {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	testdb.Exec(t, pool,
		`INSERT INTO lms.user_branch_assignments (user_id, branch_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, branchID,
	)
}

// createTempRoleWithPermissions creates a throwaway role for a test and grants it
// the requested permissions. Cleanup removes the role and mappings at test end.
func createTempRoleWithPermissions(t *testing.T, permissionNames ...string) string {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	ctx := context.Background()

	roleName := fmt.Sprintf("tmp-authz-%d", time.Now().UnixNano())
	var roleID string
	err := pool.QueryRow(ctx,
		`INSERT INTO lms.roles (name, description) VALUES ($1, $2) RETURNING id::text`,
		roleName, "temporary integration-test role",
	).Scan(&roleID)
	require.NoError(t, err, "create temporary role")

	for _, perm := range permissionNames {
		tag, err := pool.Exec(ctx,
			`INSERT INTO lms.role_permissions (role_id, permission_id)
			 SELECT $1, id FROM lms.permissions WHERE name = $2`,
			roleID, perm,
		)
		require.NoError(t, err, "grant permission %s", perm)
		require.EqualValues(t, 1, tag.RowsAffected(), "permission %s should exist in seed data", perm)
	}

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		p.Exec(context.Background(), `DELETE FROM lms.user_roles WHERE role_id = $1`, roleID)       //nolint
		p.Exec(context.Background(), `DELETE FROM lms.role_permissions WHERE role_id = $1`, roleID) //nolint
		p.Exec(context.Background(), `DELETE FROM lms.roles WHERE id = $1`, roleID)                 //nolint
	})

	return roleID
}

func assignUserRole(t *testing.T, userID, roleID string) {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	testdb.Exec(t, pool,
		`INSERT INTO lms.user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, roleID,
	)
}

// insertTestHoldingAndCopy inserts a minimal holding + copy into the specified branch
// and returns the copy UUID. Cleanup removes both rows when the test finishes.
func insertTestHoldingAndCopy(t *testing.T, branchID string) (copyID string) {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	holdingID := ""
	err := pool.QueryRow(ctx,
		`INSERT INTO lms.holdings (branch_id, title, language)
		 VALUES ($1, $2, 'en') RETURNING id`,
		branchID, "Test Holding "+suffix,
	).Scan(&holdingID)
	require.NoError(t, err, "insert test holding")

	err = pool.QueryRow(ctx,
		`INSERT INTO lms.copies (holding_id, branch_id, barcode, status_code)
		 VALUES ($1, $2, $3, 'available') RETURNING id`,
		holdingID, branchID, "TST-"+suffix,
	).Scan(&copyID)
	require.NoError(t, err, "insert test copy")

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		p.Exec(context.Background(), `DELETE FROM lms.copies WHERE id = $1`, copyID)      //nolint
		p.Exec(context.Background(), `DELETE FROM lms.holdings WHERE id = $1`, holdingID) //nolint
	})
	return copyID
}

// insertTestProgram inserts a minimal program into the specified branch and returns
// its UUID. Cleanup removes the row when the test finishes.
func insertTestProgram(t *testing.T, branchID string) (programID string) {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	ctx := context.Background()

	now := time.Now().UTC()
	err := pool.QueryRow(ctx,
		`INSERT INTO lms.programs
		   (branch_id, title, capacity, starts_at, ends_at, enrollment_channel)
		 VALUES ($1, $2, 10, $3, $4, 'any') RETURNING id`,
		branchID, "Test Program "+fmt.Sprintf("%d", now.UnixNano()),
		now.Add(time.Hour), now.Add(2*time.Hour),
	).Scan(&programID)
	require.NoError(t, err, "insert test program")

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		p.Exec(context.Background(), `DELETE FROM lms.programs WHERE id = $1`, programID) //nolint
	})
	return programID
}

// insertTestEnrollmentRule inserts a rule for the given program and returns its UUID.
func insertTestEnrollmentRule(t *testing.T, programID string) (ruleID string) {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	ctx := context.Background()

	err := pool.QueryRow(ctx,
		`INSERT INTO lms.enrollment_rules (program_id, rule_type, match_field, match_value)
		 VALUES ($1, 'whitelist', 'branch_id', 'bbbbbbbb-0000-0000-0000-000000000001')
		 RETURNING id::text`,
		programID,
	).Scan(&ruleID)
	require.NoError(t, err, "insert test enrollment rule")

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		p.Exec(context.Background(), `DELETE FROM lms.enrollment_rules WHERE id = $1`, ruleID) //nolint
	})
	return ruleID
}

// lookupReportDefinitionID returns the seeded report definition ID for the given name.
func lookupReportDefinitionID(t *testing.T, name string) string {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `SELECT id::text FROM lms.report_definitions WHERE name = $1`, name).Scan(&id)
	require.NoError(t, err, "lookup report definition")
	return id
}

// ── Circulation cross-branch IDOR ─────────────────────────────────────────────

// TestCirculation_ActiveCheckout_CrossBranch_Returns404 verifies that a user
// scoped to EAST branch cannot retrieve the active-checkout state of a copy
// that belongs to MAIN branch. The response must be 404 (not 403) to prevent
// resource enumeration.
func TestCirculation_ActiveCheckout_CrossBranch_Returns404(t *testing.T) {
	app := newAuthzTestApp(t)

	// Insert a copy into the MAIN branch.
	mainBranchID := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranchID := "bbbbbbbb-0000-0000-0000-000000000002"
	copyID := insertTestHoldingAndCopy(t, mainBranchID)

	// Create an operations_staff user assigned to the EAST branch.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "east-circ-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, eastBranchID)

	cookie := loginAs(t, app.testApp, username, "Password123!")

	// EAST user attempts to read the active checkout for a MAIN-branch copy.
	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/circulation/active/"+copyID, nil, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"EAST user must not see MAIN-branch copy active checkout (expect 404): body=%s", rec.Body.String())
}

// TestCirculation_Checkout_CrossBranch_Returns404 verifies that an EAST-branch
// user cannot checkout a copy from the MAIN branch by supplying its UUID.
func TestCirculation_Checkout_CrossBranch_Returns404(t *testing.T) {
	app := newAuthzTestApp(t)

	mainBranchID := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranchID := "bbbbbbbb-0000-0000-0000-000000000002"
	copyID := insertTestHoldingAndCopy(t, mainBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "east-checkout-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, eastBranchID)

	cookie := loginAs(t, app.testApp, username, "Password123!")

	// Attempt to checkout a MAIN-branch copy as an EAST-branch user.
	due := time.Now().UTC().Add(14 * 24 * time.Hour).Format("2006-01-02")
	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/checkout",
		map[string]any{
			"copy_id":   copyID,
			"reader_id": "00000000-0000-0000-0000-000000000099", // arbitrary reader UUID
			"due_date":  due,
		},
		cookie,
	)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"EAST user must not checkout MAIN-branch copy (expect 404): body=%s", rec.Body.String())
}

// TestCirculation_Return_CrossBranch_Returns404 verifies that an EAST-branch
// user cannot return a copy from the MAIN branch by supplying its UUID.
func TestCirculation_Return_CrossBranch_Returns404(t *testing.T) {
	app := newAuthzTestApp(t)

	mainBranchID := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranchID := "bbbbbbbb-0000-0000-0000-000000000002"
	copyID := insertTestHoldingAndCopy(t, mainBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "east-return-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, eastBranchID)

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/return",
		map[string]any{"copy_id": copyID},
		cookie,
	)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"EAST user must not return MAIN-branch copy (expect 404): body=%s", rec.Body.String())
}

// TestCirculation_AdminListAllBranches verifies that the admin (empty branchID)
// can list circulation events across all branches and receives 200.
func TestCirculation_AdminListAllBranches(t *testing.T) {
	app := newAuthzTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/circulation", nil, cookie)
	assert.Equal(t, http.StatusOK, rec.Code,
		"admin should list all-branch circulation events (200): body=%s", rec.Body.String())
}

// ── Program rules cross-branch authorization ──────────────────────────────────

// TestPrograms_Rules_CrossBranch_Returns404 verifies that a user scoped to EAST
// branch cannot list the enrollment rules of a program belonging to MAIN branch.
// The parent program lookup gates the sub-resource, so 404 is the expected response.
func TestPrograms_Rules_CrossBranch_Returns404(t *testing.T) {
	app := newAuthzTestApp(t)

	mainBranchID := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranchID := "bbbbbbbb-0000-0000-0000-000000000002"
	programID := insertTestProgram(t, mainBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "east-prog-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, eastBranchID)

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/programs/"+programID+"/rules", nil, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"EAST user must not see MAIN-branch program rules (expect 404): body=%s", rec.Body.String())
}

// TestPrograms_AddRule_CrossBranch_Returns404 verifies that an EAST-branch user
// cannot add an enrollment rule to a MAIN-branch program.
func TestPrograms_AddRule_CrossBranch_Returns404(t *testing.T) {
	app := newAuthzTestApp(t)

	mainBranchID := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranchID := "bbbbbbbb-0000-0000-0000-000000000002"
	programID := insertTestProgram(t, mainBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "east-addrule-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, eastBranchID)

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/programs/"+programID+"/rules",
		map[string]any{"rule_type": "allow", "match_field": "branch_id", "match_value": eastBranchID},
		cookie,
	)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"EAST user must not add rules to MAIN-branch program (expect 404): body=%s", rec.Body.String())
}

// TestPrograms_RemoveRule_CrossBranch_Returns404 verifies that an EAST-branch user
// cannot delete an enrollment rule from a MAIN-branch program.
func TestPrograms_RemoveRule_CrossBranch_Returns404(t *testing.T) {
	app := newAuthzTestApp(t)

	mainBranchID := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranchID := "bbbbbbbb-0000-0000-0000-000000000002"
	programID := insertTestProgram(t, mainBranchID)
	ruleID := insertTestEnrollmentRule(t, programID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "east-removerule-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, eastBranchID)

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodDelete,
		"/api/v1/programs/"+programID+"/rules/"+ruleID, nil, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"EAST user must not remove rules from MAIN-branch program (expect 404): body=%s", rec.Body.String())
}

// TestPrograms_AdminListAllBranches verifies that the admin (empty branchID) can
// list programs across all branches and receives 200.
func TestPrograms_AdminListAllBranches(t *testing.T) {
	app := newAuthzTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/programs", nil, cookie)
	assert.Equal(t, http.StatusOK, rec.Code,
		"admin should list all-branch programs (200): body=%s", rec.Body.String())
}

// ── Unassigned-user sentinel UUID ─────────────────────────────────────────────

// TestUsers_UnassignedBranch_ReaderListIsEmpty verifies that an operations_staff
// user with no branch assignment receives an empty reader list rather than seeing
// all readers across every branch.
//
// When a non-admin user has no branch assignments the BranchScope middleware sets
// branchID to the sentinel UUID "00000000-0000-0000-0000-000000000000". The reader
// repository scopes its query to WHERE branch_id = <sentinel>, which matches no
// real branch → empty result set. This confirms the sentinel is not downgraded to
// "" (global scope) anywhere in the request path.
func TestUsers_UnassignedBranch_ReaderListIsEmpty(t *testing.T) {
	app := newAuthzTestApp(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "unassigned-" + suffix
	// Create operations_staff user with NO branch assignment.
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	// The user has readers:read permission, so 200 is expected.
	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/readers", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"unassigned user with readers:read should get 200 on GET /readers: body=%s", rec.Body.String())

	// The result set must be empty — sentinel UUID matches no real branch.
	var resp struct {
		Items []any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items,
		"unassigned user must see 0 readers (sentinel UUID matches no branch): got %d", len(resp.Items))
}

// TestUsers_UnassignedBranch_UserListIsEmpty verifies that a non-admin staff user
// with users:read but no branch assignment cannot see all users via the sentinel UUID path.
func TestUsers_UnassignedBranch_UserListIsEmpty(t *testing.T) {
	app := newAuthzTestApp(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "unassigned-users-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")
	roleID := createTempRoleWithPermissions(t, "users:read")
	assignUserRole(t, userID, roleID)

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/users", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"unassigned user with users:read should get 200 on GET /users: body=%s", rec.Body.String())

	var resp struct {
		Items []any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items,
		"unassigned user must see 0 users (sentinel UUID matches no branch): got %d", len(resp.Items))
}

// TestUsers_GetUser_CrossBranch_Returns404 verifies that non-admin callers with
// users:read cannot fetch a user assigned only to another branch.
func TestUsers_GetUser_CrossBranch_Returns404(t *testing.T) {
	app := newAuthzTestApp(t)

	mainBranchID := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranchID := "bbbbbbbb-0000-0000-0000-000000000002"

	callerName := "users-read-main-" + fmt.Sprintf("%d", time.Now().UnixNano())
	callerID := createTestUser(t, app.testApp, callerName, callerName+"@test.local", "Password123!", "")
	assignUserToBranch(t, callerID, mainBranchID)
	assignUserRole(t, callerID, createTempRoleWithPermissions(t, "users:read"))

	targetName := "users-target-east-" + fmt.Sprintf("%d", time.Now().UnixNano())
	targetID := createTestUser(t, app.testApp, targetName, targetName+"@test.local", "Password123!", "")
	assignUserToBranch(t, targetID, eastBranchID)

	cookie := loginAs(t, app.testApp, callerName, "Password123!")
	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/users/"+targetID, nil, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"cross-branch GET /users/:id must return 404: body=%s", rec.Body.String())
}

// TestUsers_UpdateUser_CrossBranch_Returns404 verifies that non-admin callers
// with users:write cannot update a user assigned only to another branch.
func TestUsers_UpdateUser_CrossBranch_Returns404(t *testing.T) {
	app := newAuthzTestApp(t)

	mainBranchID := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranchID := "bbbbbbbb-0000-0000-0000-000000000002"

	callerName := "users-write-main-" + fmt.Sprintf("%d", time.Now().UnixNano())
	callerID := createTestUser(t, app.testApp, callerName, callerName+"@test.local", "Password123!", "")
	assignUserToBranch(t, callerID, mainBranchID)
	assignUserRole(t, callerID, createTempRoleWithPermissions(t, "users:write"))

	targetName := "users-update-east-" + fmt.Sprintf("%d", time.Now().UnixNano())
	targetID := createTestUser(t, app.testApp, targetName, targetName+"@test.local", "Password123!", "")
	assignUserToBranch(t, targetID, eastBranchID)

	cookie := loginAs(t, app.testApp, callerName, "Password123!")
	rec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/users/"+targetID, map[string]any{
		"email": "updated@test.local",
	}, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"cross-branch PATCH /users/:id must return 404: body=%s", rec.Body.String())
}

// TestReports_AdminRunWithBranchParam_Returns200 verifies that administrator
// callers can run a report by providing an explicit branch_id query param even
// though their context branch scope is empty.
func TestReports_AdminRunWithBranchParam_Returns200(t *testing.T) {
	app := newAuthzTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	definitionID := lookupReportDefinitionID(t, "circulation_overview")
	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/reports/run?definition_id="+definitionID+
			"&branch_id=bbbbbbbb-0000-0000-0000-000000000001&from=2026-01-01&to=2026-01-31",
		nil,
		cookie,
	)
	assert.Equal(t, http.StatusOK, rec.Code,
		"admin should run branch-scoped report with explicit branch_id (200): body=%s", rec.Body.String())
}

// TestReports_AdminRecalculateAllBranches_Returns200 verifies that the admin's
// empty branch scope intentionally maps to all-branches recalculation.
func TestReports_AdminRecalculateAllBranches_Returns200(t *testing.T) {
	app := newAuthzTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/reports/recalculate",
		map[string]any{"from": "2026-01-01", "to": "2026-01-01"},
		cookie,
	)
	assert.Equal(t, http.StatusOK, rec.Code,
		"admin should recalculate all-branch aggregates with empty branch scope (200): body=%s", rec.Body.String())
}
