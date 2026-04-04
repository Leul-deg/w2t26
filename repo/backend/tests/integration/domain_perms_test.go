package integration

// domain_perms_test.go verifies permission enforcement for holdings, stocktake,
// imports, exports, and reports routes via a full wired test application.

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/internal/apierr"
	auditpkg "lms/internal/audit"
	"lms/internal/domain/exports"
	"lms/internal/domain/holdings"
	"lms/internal/domain/imports"
	"lms/internal/domain/reports"
	"lms/internal/domain/stocktake"
	"lms/internal/domain/users"
	copies "lms/internal/domain/copies"
	appmw "lms/internal/middleware"
	"lms/internal/store/postgres"
	"lms/tests/testdb"
)

// fullTestApp extends testApp with additional domain services.
type fullTestApp struct {
	*testApp
}

// newFullTestApp creates an Echo app with all domain routes registered.
func newFullTestApp(t *testing.T) *fullTestApp {
	t.Helper()
	pool := testdb.Open(t)
	t.Cleanup(func() { pool.Close() })

	// Core repos.
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

	// Auth routes.
	authHandler := users.NewHandler(authService)
	authHandler.RegisterRoutes(api, requireAuth)

	// Holdings + copies routes.
	holdingRepo := postgres.NewHoldingRepo(pool)
	copyRepo := postgres.NewCopyRepo(pool)
	holdingService := holdings.NewService(holdingRepo, copyRepo, auditLogger)
	holdingsHandler := holdings.NewHandler(holdingService)
	holdingsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// Stocktake routes.
	stocktakeRepo := postgres.NewStocktakeRepo(pool)
	stocktakeService := stocktake.NewService(stocktakeRepo, copies.Repository(copyRepo), auditLogger)
	stocktakeHandler := stocktake.NewHandler(stocktakeService)
	stocktakeHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// Imports routes.
	importRepo := postgres.NewImportRepo(pool)
	importService := imports.NewService(importRepo, pool, auditLogger)
	importsHandler := imports.NewHandler(importService)
	importsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// Exports routes.
	exportRepo := postgres.NewExportRepo(pool)
	exportService := exports.NewService(exportRepo, pool, auditLogger)
	exportsHandler := exports.NewHandler(exportService)
	exportsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// Reports routes.
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
	return &fullTestApp{testApp: base}
}

// ── Holdings ──────────────────────────────────────────────────────────────────

// TestHoldings_ListRequiresPermission verifies that a user without holdings:read
// receives 403 when accessing GET /holdings.
func TestHoldings_ListRequiresPermission(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-holdings-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/holdings", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without holdings:read should get 403: body=%s", rec.Body.String())
}

// TestHoldings_CreateRequiresPermission verifies that a user without holdings:write
// receives 403 when attempting POST /holdings.
func TestHoldings_CreateRequiresPermission(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-hcreate-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/holdings",
		map[string]any{"title": "Test Book", "branch_id": "bbbbbbbb-0000-0000-0000-000000000001"},
		cookie,
	)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without holdings:write should get 403: body=%s", rec.Body.String())
}

// TestHoldings_AdminCanList verifies that admin (has holdings:read) gets 200.
func TestHoldings_AdminCanList(t *testing.T) {
	app := newFullTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/holdings", nil, cookie)
	assert.Equal(t, http.StatusOK, rec.Code,
		"admin should get 200 on GET /holdings: body=%s", rec.Body.String())
}

// TestHoldings_OperationsStaffCanList verifies that operations_staff (has holdings:read)
// gets 200.
func TestHoldings_OperationsStaffCanList(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-holdings-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/holdings", nil, cookie)
	assert.Equal(t, http.StatusOK, rec.Code,
		"operations_staff should get 200 on GET /holdings: body=%s", rec.Body.String())
}

// TestHoldings_ContentModeratorCannotList verifies that content_moderator (no holdings:read)
// receives 403 — confirming that cross-role escalation is not possible.
func TestHoldings_ContentModeratorCannotList(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "mod-holdings-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/holdings", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator has no holdings:read — should get 403: body=%s", rec.Body.String())
}

// ── Stocktake ─────────────────────────────────────────────────────────────────

// TestStocktake_ListRequiresPermission verifies that a user without stocktake:read
// receives 403 on GET /stocktake.
func TestStocktake_ListRequiresPermission(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-stk-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/stocktake", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without stocktake:read should get 403: body=%s", rec.Body.String())
}

// TestStocktake_CreateRequiresPermission verifies that a user without stocktake:write
// receives 403 on POST /stocktake.
func TestStocktake_CreateRequiresPermission(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-stkw-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake",
		map[string]any{"branch_id": "bbbbbbbb-0000-0000-0000-000000000001", "name": "Test Session"},
		cookie,
	)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without stocktake:write should get 403: body=%s", rec.Body.String())
}

// TestStocktake_AdminCanList verifies that admin (has stocktake:read) gets 200.
func TestStocktake_AdminCanList(t *testing.T) {
	app := newFullTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/stocktake", nil, cookie)
	assert.Equal(t, http.StatusOK, rec.Code,
		"admin should get 200 on GET /stocktake: body=%s", rec.Body.String())
}

// TestStocktake_ContentModeratorCannotCreate verifies that content_moderator
// (no stocktake:write) cannot create a session — confirming no cross-role escalation.
func TestStocktake_ContentModeratorCannotCreate(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "mod-stk-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake",
		map[string]any{"branch_id": "bbbbbbbb-0000-0000-0000-000000000001", "name": "Test"},
		cookie,
	)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator has no stocktake:write — should get 403: body=%s", rec.Body.String())
}

// ── Imports ───────────────────────────────────────────────────────────────────

// TestImports_ListRequiresPermission verifies that a user without imports:create
// receives 403 on GET /imports.
func TestImports_ListRequiresPermission(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-imp-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without imports:create should get 403 on GET /imports: body=%s", rec.Body.String())
}

// TestImports_UploadRequiresPermission verifies that a user without imports:create
// receives 403 on POST /imports (upload).
func TestImports_UploadRequiresPermission(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	// content_moderator has no imports:create permission.
	username := "mod-imp-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	// The upload endpoint reads multipart, but the permission check happens before
	// the file is parsed — so any request body produces 403, not 400.
	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/imports", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator has no imports:create — should get 403: body=%s", rec.Body.String())
}

// TestImports_AdminCanList verifies that admin (has imports:create) gets 200.
func TestImports_AdminCanList(t *testing.T) {
	app := newFullTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports", nil, cookie)
	assert.Equal(t, http.StatusOK, rec.Code,
		"admin should get 200 on GET /imports: body=%s", rec.Body.String())
}

// TestImports_OperationsStaffCanList verifies that operations_staff (has imports:create)
// can list import jobs.
func TestImports_OperationsStaffCanList(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-imp-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports", nil, cookie)
	assert.Equal(t, http.StatusOK, rec.Code,
		"operations_staff should get 200 on GET /imports: body=%s", rec.Body.String())
}

// ── Exports ───────────────────────────────────────────────────────────────────

// TestExports_ListRequiresPermission verifies that a user without exports:create
// receives 403 on GET /exports.
func TestExports_ListRequiresPermission(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-exp-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/exports", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without exports:create should get 403: body=%s", rec.Body.String())
}

// TestExports_ContentModeratorCannotList verifies that content_moderator (no exports:create)
// cannot list exports — confirming no cross-role escalation.
func TestExports_ContentModeratorCannotList(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "mod-exp-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/exports", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator has no exports:create — should get 403: body=%s", rec.Body.String())
}

// TestExports_AdminCanList verifies that admin (has exports:create) gets 200.
func TestExports_AdminCanList(t *testing.T) {
	app := newFullTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/exports", nil, cookie)
	assert.Equal(t, http.StatusOK, rec.Code,
		"admin should get 200 on GET /exports: body=%s", rec.Body.String())
}

// ── Reports ───────────────────────────────────────────────────────────────────

// TestReports_ListDefinitionsRequiresRead verifies that a user without reports:read
// receives 403 on GET /reports/definitions.
func TestReports_ListDefinitionsRequiresRead(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-rep-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/reports/definitions", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without reports:read should get 403: body=%s", rec.Body.String())
}

// TestReports_AdminCanListDefinitions verifies that admin gets 200 and the seeded
// report definitions are present.
func TestReports_AdminCanListDefinitions(t *testing.T) {
	app := newFullTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/reports/definitions", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"admin should get 200 on GET /reports/definitions: body=%s", rec.Body.String())

	// Seeded definitions include program_utilization and enrollment_mix.
	body := rec.Body.String()
	assert.Contains(t, body, "program_utilization", "seeded program_utilization definition must be present")
	assert.Contains(t, body, "enrollment_mix", "seeded enrollment_mix definition must be present")
}

// TestReports_RunReportRequiresRead verifies that a user without reports:read
// receives 403 on GET /reports/run.
func TestReports_RunReportRequiresRead(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-run-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/reports/run?definition_id=any", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without reports:read should get 403 on run: body=%s", rec.Body.String())
}

// TestReports_OperationsStaffCanListDefinitions verifies that operations_staff
// (has reports:read) can access report definitions.
func TestReports_OperationsStaffCanListDefinitions(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-rep-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/reports/definitions", nil, cookie)
	assert.Equal(t, http.StatusOK, rec.Code,
		"operations_staff should get 200 on GET /reports/definitions: body=%s", rec.Body.String())
}
