package apitests

// domain_perms_test.go verifies permission enforcement for holdings, stocktake,
// imports, exports, and reports routes via a full wired test application.

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
	"lms/internal/domain/appeals"
	copies "lms/internal/domain/copies"
	"lms/internal/domain/exports"
	"lms/internal/domain/feedback"
	"lms/internal/domain/holdings"
	"lms/internal/domain/imports"
	"lms/internal/domain/reports"
	"lms/internal/domain/stocktake"
	"lms/internal/domain/users"
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

	// Feedback routes.
	feedbackRepo := postgres.NewFeedbackRepo(pool)
	feedbackService := feedback.NewService(feedbackRepo, auditLogger)
	feedbackHandler := feedback.NewHandler(feedbackService)
	feedbackHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// Appeals routes.
	appealsRepo := postgres.NewAppealsRepo(pool)
	appealsService := appeals.NewService(appealsRepo, auditLogger)
	appealsHandler := appeals.NewHandler(appealsService)
	appealsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

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

func insertTestReaderInBranch(t *testing.T, branchID string) string {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	var readerID string
	err := pool.QueryRow(ctx, `
		INSERT INTO lms.readers (branch_id, reader_number, first_name, last_name, status_code)
		VALUES ($1, $2, 'Fixture', 'Reader', 'active')
		RETURNING id::text`,
		branchID, "PERM-"+suffix,
	).Scan(&readerID)
	require.NoError(t, err)

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		p.Exec(context.Background(), `DELETE FROM lms.readers WHERE id = $1`, readerID) //nolint
	})
	return readerID
}

func insertTestHoldingInBranch(t *testing.T, branchID string) string {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	var holdingID string
	err := pool.QueryRow(ctx, `
		INSERT INTO lms.holdings (branch_id, title, language)
		VALUES ($1, $2, 'en')
		RETURNING id::text`,
		branchID, "Holding "+suffix,
	).Scan(&holdingID)
	require.NoError(t, err)

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		p.Exec(context.Background(), `DELETE FROM lms.copies WHERE holding_id = $1`, holdingID) //nolint
		p.Exec(context.Background(), `DELETE FROM lms.holdings WHERE id = $1`, holdingID)       //nolint
	})
	return holdingID
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

// TestHoldings_AddCopy_CrossBranch_Returns404 verifies that a user scoped to one
// branch cannot attach a copy to a holding from another branch by guessing the holding UUID.
func TestHoldings_AddCopy_CrossBranch_Returns404(t *testing.T) {
	app := newFullTestApp(t)
	mainBranchID := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranchID := "bbbbbbbb-0000-0000-0000-000000000002"
	holdingID := insertTestHoldingInBranch(t, mainBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "east-addcopy-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, eastBranchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/holdings/"+holdingID+"/copies", map[string]any{
		"barcode":     "XCOPY-" + suffix,
		"status_code": "available",
		"condition":   "good",
	}, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"cross-branch POST /holdings/:id/copies must return 404: body=%s", rec.Body.String())
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

// TestReports_RunReport_ReturnsDefinitionAndRows verifies the actual run endpoint
// returns a concrete JSON payload for an operations staff user in their assigned branch.
func TestReports_RunReport_ReturnsDefinitionAndRows(t *testing.T) {
	app := newFullTestApp(t)
	const branchID = "bbbbbbbb-0000-0000-0000-000000000001"
	definitionID := lookupReportDefinitionID(t, "reader_activity")

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-run-report-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, branchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/reports/run?definition_id="+definitionID+"&from=2026-01-01&to=2026-01-31",
		nil,
		cookie,
	)
	require.Equal(t, http.StatusOK, rec.Code,
		"operations_staff should run reader_activity report successfully: body=%s", rec.Body.String())

	var resp struct {
		Definition struct {
			Name string `json:"name"`
		} `json:"definition"`
		Rows     []map[string]any `json:"rows"`
		RowCount int              `json:"row_count"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "reader_activity", resp.Definition.Name)
	assert.Len(t, resp.Rows, 3)
	assert.Equal(t, 3, resp.RowCount)
}

// TestReports_Export_ReturnsCSVAndAuditHeader verifies the real export endpoint
// returns a CSV payload plus the export job header for an authorized branch-scoped user.
func TestReports_Export_ReturnsCSVAndAuditHeader(t *testing.T) {
	app := newFullTestApp(t)
	const branchID = "bbbbbbbb-0000-0000-0000-000000000001"
	definitionID := lookupReportDefinitionID(t, "reader_activity")

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-export-report-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, branchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/reports/export?definition_id="+definitionID+"&from=2026-01-01&to=2026-01-31",
		nil,
		cookie,
	)
	require.Equal(t, http.StatusOK, rec.Code,
		"operations_staff should export reader_activity report successfully: body=%s", rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment; filename=")
	assert.NotEmpty(t, rec.Header().Get("X-Export-Job-ID"))
	assert.Contains(t, rec.Body.String(), "metric,value")
}

// ── Feedback / appeals submit permissions ─────────────────────────────────────

func TestFeedback_SubmitRequiresPermission(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-feedback-submit-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/feedback", map[string]any{
		"reader_id":   "00000000-0000-0000-0000-000000000011",
		"target_type": "program",
		"target_id":   "00000000-0000-0000-0000-000000000012",
		"rating":      5,
	}, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without feedback:submit should get 403 on POST /feedback: body=%s", rec.Body.String())
}

func TestFeedback_OperationsStaffCanSubmit(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-feedback-submit-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	const branchID = "bbbbbbbb-0000-0000-0000-000000000001"
	assignUserToBranch(t, userID, branchID)
	readerID := insertTestReaderInBranch(t, branchID)

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/feedback", map[string]any{
		"reader_id":   readerID,
		"target_type": "program",
		"target_id":   "00000000-0000-0000-0000-000000000022",
		"rating":      4,
	}, cookie)
	assert.Equal(t, http.StatusCreated, rec.Code,
		"operations_staff should get 201 on POST /feedback: body=%s", rec.Body.String())
}

func TestAppeals_SubmitRequiresPermission(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-appeal-submit-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/appeals", map[string]any{
		"reader_id":   "00000000-0000-0000-0000-000000000031",
		"appeal_type": "other",
		"reason":      "Need manual review",
	}, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without appeals:submit should get 403 on POST /appeals: body=%s", rec.Body.String())
}

func TestAppeals_OperationsStaffCanSubmit(t *testing.T) {
	app := newFullTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-appeal-submit-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	const branchID = "bbbbbbbb-0000-0000-0000-000000000001"
	assignUserToBranch(t, userID, branchID)
	readerID := insertTestReaderInBranch(t, branchID)

	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/appeals", map[string]any{
		"reader_id":   readerID,
		"appeal_type": "other",
		"reason":      "Need manual review",
	}, cookie)
	assert.Equal(t, http.StatusCreated, rec.Code,
		"operations_staff should get 201 on POST /appeals: body=%s", rec.Body.String())
}
