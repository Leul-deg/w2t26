package apitests

// exports_imports_reports_test.go covers the HTTP endpoints that were missing
// true no-mock HTTP-layer tests:
//
//   POST /api/v1/exports/readers         (ExportReaders — previously only tested via direct handler call)
//   POST /api/v1/exports/holdings        (ExportHoldings — no test at all)
//   GET  /api/v1/imports/template/:type  (DownloadTemplate — no test at all)
//   GET  /api/v1/reports/aggregates      (ListAggregates — no test at all)

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opsUserWithBranch creates an operations_staff user assigned to mainBranchID
// and returns a session cookie. Used across several tests below.
func opsUserWithBranch(t *testing.T, app *completeTestApp) *http.Cookie {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-exp-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, mainBranchID)
	return loginAs(t, app.testApp, username, "Password123!")
}

// ── POST /api/v1/exports/readers ─────────────────────────────────────────────

// TestExports_ExportReaders_ReturnsCSV verifies that a user with exports:create
// can trigger POST /exports/readers and receive a CSV file attachment.
func TestExports_ExportReaders_ReturnsCSV(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := opsUserWithBranch(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/exports/readers", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"exports:create user should trigger readers export: body=%s", rec.Body.String())

	// Response must be a file download.
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment",
		"response must have Content-Disposition: attachment")
	assert.NotEmpty(t, rec.Header().Get("X-Export-Job-ID"),
		"response must include X-Export-Job-ID header")
	assert.True(t,
		strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") ||
			strings.HasPrefix(rec.Header().Get("Content-Type"), "application/"),
		"content-type must be csv or spreadsheet: got %s", rec.Header().Get("Content-Type"))
}

// TestExports_ExportReaders_RequiresPermission verifies that a user without
// exports:create receives 403 on POST /exports/readers.
func TestExports_ExportReaders_RequiresPermission(t *testing.T) {
	app := newCompleteTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-export-readers-" + suffix
	// content_moderator has no exports:create permission.
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/exports/readers", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator should get 403 on POST /exports/readers: body=%s", rec.Body.String())
}

// ── POST /api/v1/exports/holdings ────────────────────────────────────────────

// TestExports_ExportHoldings_ReturnsCSV verifies that a user with exports:create
// can trigger POST /exports/holdings and receive a file attachment.
func TestExports_ExportHoldings_ReturnsCSV(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := opsUserWithBranch(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/exports/holdings", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"exports:create user should trigger holdings export: body=%s", rec.Body.String())

	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment",
		"response must have Content-Disposition: attachment")
	assert.NotEmpty(t, rec.Header().Get("X-Export-Job-ID"),
		"response must include X-Export-Job-ID header")
}

// TestExports_ExportHoldings_RequiresPermission verifies that a user without
// exports:create receives 403 on POST /exports/holdings.
func TestExports_ExportHoldings_RequiresPermission(t *testing.T) {
	app := newCompleteTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-export-holdings-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/exports/holdings", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator should get 403 on POST /exports/holdings: body=%s", rec.Body.String())
}

// ── GET /api/v1/imports/template/:type ───────────────────────────────────────

// TestImports_DownloadTemplate_Readers verifies that a user with imports:create
// can download the readers import CSV template (header row only).
func TestImports_DownloadTemplate_Readers(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports/template/readers", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"imports:create user should download readers template: body=%s", rec.Body.String())

	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment",
		"template response must be a file download")
	// The template must contain the reader_number column header.
	assert.Contains(t, rec.Body.String(), "reader_number",
		"readers template CSV must include 'reader_number' column")
}

// TestImports_DownloadTemplate_Holdings verifies that the holdings template
// downloads successfully with the expected column headers.
func TestImports_DownloadTemplate_Holdings(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports/template/holdings", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"imports:create user should download holdings template: body=%s", rec.Body.String())

	assert.Contains(t, rec.Body.String(), "title",
		"holdings template CSV must include 'title' column")
}

// TestImports_DownloadTemplate_UnknownType verifies that an unknown import type
// returns 422.
func TestImports_DownloadTemplate_UnknownType(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports/template/nonexistent", nil, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"unknown import type should return 422: body=%s", rec.Body.String())
}

// TestImports_DownloadTemplate_RequiresPermission verifies that a user without
// imports:preview or imports:create receives 403.
func TestImports_DownloadTemplate_RequiresPermission(t *testing.T) {
	app := newCompleteTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-tmpl-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports/template/readers", nil, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator should get 403 on GET /imports/template/:type: body=%s", rec.Body.String())
}

// ── GET /api/v1/reports/aggregates ───────────────────────────────────────────

// TestReports_ListAggregates_AdminReturns200 verifies that admin can call
// GET /reports/aggregates with a branch_id and date range and receive 200.
func TestReports_ListAggregates_AdminReturns200(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/reports/aggregates?branch_id=bbbbbbbb-0000-0000-0000-000000000001&from=2026-01-01&to=2026-01-31",
		nil, cookie,
	)
	require.Equal(t, http.StatusOK, rec.Code,
		"admin should list aggregates (200): body=%s", rec.Body.String())

	// Response must be valid JSON (array, object, or null for an empty result set).
	body := rec.Body.Bytes()
	assert.True(t, len(body) > 0, "response body must not be empty")
	var result any
	assert.NoError(t, json.Unmarshal(body, &result), "response must be valid JSON")
}

// TestReports_ListAggregates_RequiresPermission verifies that a user without
// reports:read receives 403.
func TestReports_ListAggregates_RequiresPermission(t *testing.T) {
	app := newCompleteTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-agg-" + suffix
	// Create a user with no roles — guarantees no reports:read.
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")
	assignUserToBranch(t, userID, mainBranchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/reports/aggregates?branch_id=bbbbbbbb-0000-0000-0000-000000000001&from=2026-01-01&to=2026-01-31",
		nil, cookie,
	)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without reports:read should get 403 on GET /reports/aggregates: body=%s", rec.Body.String())
}

// TestReports_ListAggregates_MissingDates verifies that omitting from/to date
// params returns 422.
func TestReports_ListAggregates_MissingDates(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/reports/aggregates?branch_id=bbbbbbbb-0000-0000-0000-000000000001",
		nil, cookie,
	)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"missing date range should return 422: body=%s", rec.Body.String())
}

// TestReports_ListAggregates_OperationsStaffReturns200 verifies that
// operations_staff (which has reports:read) can also list aggregates.
func TestReports_ListAggregates_OperationsStaffReturns200(t *testing.T) {
	app := newCompleteTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "ops-agg-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, mainBranchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/reports/aggregates?from=2026-01-01&to=2026-01-31",
		nil, cookie,
	)
	require.Equal(t, http.StatusOK, rec.Code,
		"operations_staff should list aggregates (200): body=%s", rec.Body.String())
}

// TestReports_ListAggregates_DefinitionFilter verifies that passing a
// definition_id filter is accepted and returns 200 (not an error).
func TestReports_ListAggregates_DefinitionFilter(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	definitionID := lookupReportDefinitionID(t, "circulation_overview")

	rec := doRequest(t, app.testApp, http.MethodGet,
		fmt.Sprintf(
			"/api/v1/reports/aggregates?branch_id=bbbbbbbb-0000-0000-0000-000000000001&definition_id=%s&from=2026-01-01&to=2026-01-31",
			definitionID,
		),
		nil, cookie,
	)
	require.Equal(t, http.StatusOK, rec.Code,
		"filtered aggregates request should return 200: body=%s", rec.Body.String())

	// Validate response is a valid JSON structure.
	var resp any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
}
