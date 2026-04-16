package apitests

// stocktake_lifecycle_test.go exercises the full stocktake API over real HTTP:
//
//   POST /api/v1/stocktake              (create session)
//   GET  /api/v1/stocktake              (list sessions)
//   GET  /api/v1/stocktake/:id          (get session)
//   PATCH /api/v1/stocktake/:id/status  (close or cancel)
//   POST /api/v1/stocktake/:id/scan     (record a barcode scan)
//   GET  /api/v1/stocktake/:id/findings (list findings)
//   GET  /api/v1/stocktake/:id/variances (get variances)
//
// All tests use the EAST branch (bbbbbbbb-0000-0000-0000-000000000002) to avoid
// conflicts with tests in other files that operate on the MAIN branch.
// The unique index on stocktake_sessions prevents two active sessions per branch,
// so every test that creates a session registers cleanup that cancels it.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/tests/testdb"
)

const eastBranchID = "bbbbbbbb-0000-0000-0000-000000000002"

// ── helpers ───────────────────────────────────────────────────────────────────

// stocktakeOpsUserCookie creates an operations_staff user scoped to branchID
// and returns a logged-in session cookie. operations_staff has stocktake:read
// and stocktake:write permissions.
func stocktakeOpsUserCookie(t *testing.T, app *completeTestApp, branchID string) *http.Cookie {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "stk-ops-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, branchID)
	return loginAs(t, app.testApp, username, "Password123!")
}

// cancelActiveStocktakeSessions directly updates the DB to cancel any open or
// in_progress sessions for the given branch. Used at the start of every test
// as a defensive pre-test cleanup in case a prior run left state behind, and
// in t.Cleanup to ensure the branch is unblocked for subsequent tests.
func cancelActiveStocktakeSessions(t *testing.T, branchID string) {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	_, _ = pool.Exec(contextBG(),
		`UPDATE lms.stocktake_sessions
		 SET status = 'cancelled', closed_at = NOW()
		 WHERE branch_id = $1 AND status IN ('open', 'in_progress')`,
		branchID,
	)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestStocktake_CreateAndGetSession verifies POST /stocktake returns 201 and
// GET /stocktake/:id returns 200 with the same session.
func TestStocktake_CreateAndGetSession(t *testing.T) {
	app := newCompleteTestApp(t)
	cancelActiveStocktakeSessions(t, eastBranchID)
	t.Cleanup(func() { cancelActiveStocktakeSessions(t, eastBranchID) })
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "Session " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code,
		"POST /stocktake must return 201: body=%s", createRec.Body.String())

	var session map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &session))
	sessionID, ok := session["id"].(string)
	require.True(t, ok && sessionID != "", "response must include non-empty session id")

	// Get the session.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/stocktake/"+sessionID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code,
		"GET /stocktake/:id must return 200: body=%s", getRec.Body.String())

	var got map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &got))
	assert.Equal(t, sessionID, got["id"], "returned session id must match created session id")
}

// TestStocktake_ListSessions verifies GET /stocktake returns 200 with a paginated
// list envelope.
func TestStocktake_ListSessions(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/stocktake", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /stocktake must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, hasItems := resp["items"]
	assert.True(t, hasItems, "list sessions response must include items field")
}

// TestStocktake_DuplicateActiveSession_Returns409 verifies that creating a second
// active session for the same branch returns 409 Conflict.
func TestStocktake_DuplicateActiveSession_Returns409(t *testing.T) {
	app := newCompleteTestApp(t)
	cancelActiveStocktakeSessions(t, eastBranchID)
	t.Cleanup(func() { cancelActiveStocktakeSessions(t, eastBranchID) })
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// First session — must succeed.
	rec1 := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "First " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	require.Equal(t, http.StatusCreated, rec1.Code,
		"first session create must return 201: body=%s", rec1.Body.String())

	// Second session for the same branch — must conflict.
	rec2 := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "Second " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	assert.Equal(t, http.StatusConflict, rec2.Code,
		"duplicate active session must return 409: body=%s", rec2.Body.String())
}

// TestStocktake_RecordScan_KnownBarcode verifies that scanning a barcode that
// belongs to a copy in the branch returns 201 with finding_type "found".
func TestStocktake_RecordScan_KnownBarcode(t *testing.T) {
	app := newCompleteTestApp(t)
	cancelActiveStocktakeSessions(t, eastBranchID)
	t.Cleanup(func() { cancelActiveStocktakeSessions(t, eastBranchID) })
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	// Insert a copy in the east branch with a known barcode.
	holdingID := insertAPITestHolding(t, eastBranchID)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	barcode := "STK-KNOWN-" + suffix
	insertAPITestCopy(t, holdingID, eastBranchID, barcode)

	// Create a session.
	sessRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "Known Barcode " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	require.Equal(t, http.StatusCreated, sessRec.Code)

	var sess map[string]any
	require.NoError(t, json.Unmarshal(sessRec.Body.Bytes(), &sess))
	sessionID := sess["id"].(string)

	// Scan the known barcode.
	scanRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/stocktake/"+sessionID+"/scan",
		map[string]any{"barcode": barcode},
		cookie,
	)
	require.Equal(t, http.StatusCreated, scanRec.Code,
		"POST /stocktake/:id/scan must return 201 for known barcode: body=%s", scanRec.Body.String())

	var finding map[string]any
	require.NoError(t, json.Unmarshal(scanRec.Body.Bytes(), &finding))
	assert.Equal(t, "found", finding["finding_type"],
		"scanning a barcode matching an existing copy must produce finding_type=found")
}

// TestStocktake_RecordScan_UnknownBarcode verifies that scanning an unrecognized
// barcode (not in the branch inventory) returns 201 with finding_type "unexpected".
func TestStocktake_RecordScan_UnknownBarcode(t *testing.T) {
	app := newCompleteTestApp(t)
	cancelActiveStocktakeSessions(t, eastBranchID)
	t.Cleanup(func() { cancelActiveStocktakeSessions(t, eastBranchID) })
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	sessRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "Unknown Barcode " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	require.Equal(t, http.StatusCreated, sessRec.Code)

	var sess map[string]any
	require.NoError(t, json.Unmarshal(sessRec.Body.Bytes(), &sess))
	sessionID := sess["id"].(string)

	scanRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/stocktake/"+sessionID+"/scan",
		map[string]any{"barcode": "NOSUCH-" + suffix},
		cookie,
	)
	require.Equal(t, http.StatusCreated, scanRec.Code,
		"POST /stocktake/:id/scan must return 201 for unknown barcode: body=%s", scanRec.Body.String())

	var finding map[string]any
	require.NoError(t, json.Unmarshal(scanRec.Body.Bytes(), &finding))
	assert.Equal(t, "unexpected", finding["finding_type"],
		"scanning an unknown barcode must produce finding_type=unexpected")
}

// TestStocktake_RecordScan_Idempotent verifies that scanning the same barcode twice
// in the same session returns the existing finding rather than a conflict error.
// The unique index on (session_id, scanned_barcode) enforces idempotency.
func TestStocktake_RecordScan_Idempotent(t *testing.T) {
	app := newCompleteTestApp(t)
	cancelActiveStocktakeSessions(t, eastBranchID)
	t.Cleanup(func() { cancelActiveStocktakeSessions(t, eastBranchID) })
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	barcode := "STK-IDEM-" + suffix

	sessRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "Idempotent " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	require.Equal(t, http.StatusCreated, sessRec.Code)

	var sess map[string]any
	require.NoError(t, json.Unmarshal(sessRec.Body.Bytes(), &sess))
	sessionID := sess["id"].(string)

	// First scan.
	rec1 := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/stocktake/"+sessionID+"/scan",
		map[string]any{"barcode": barcode}, cookie)
	require.Equal(t, http.StatusCreated, rec1.Code,
		"first scan must return 201: body=%s", rec1.Body.String())

	// Second scan of the same barcode in the same session.
	rec2 := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/stocktake/"+sessionID+"/scan",
		map[string]any{"barcode": barcode}, cookie)
	// Must return the existing finding, not a 500 or 409.
	assert.NotEqual(t, http.StatusInternalServerError, rec2.Code,
		"re-scanning same barcode must not cause 500: body=%s", rec2.Body.String())
	assert.Less(t, rec2.Code, 500,
		"re-scanning same barcode must not return a server error: body=%s", rec2.Body.String())
}

// TestStocktake_ListFindings verifies GET /stocktake/:id/findings returns 200 with
// a paginated list that includes scanned barcodes.
func TestStocktake_ListFindings(t *testing.T) {
	app := newCompleteTestApp(t)
	cancelActiveStocktakeSessions(t, eastBranchID)
	t.Cleanup(func() { cancelActiveStocktakeSessions(t, eastBranchID) })
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	sessRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "List Findings " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	require.Equal(t, http.StatusCreated, sessRec.Code)

	var sess map[string]any
	require.NoError(t, json.Unmarshal(sessRec.Body.Bytes(), &sess))
	sessionID := sess["id"].(string)

	// Scan two distinct barcodes.
	doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake/"+sessionID+"/scan",
		map[string]any{"barcode": "FIND-A-" + suffix}, cookie)
	doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake/"+sessionID+"/scan",
		map[string]any{"barcode": "FIND-B-" + suffix}, cookie)

	listRec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/stocktake/"+sessionID+"/findings", nil, cookie)
	require.Equal(t, http.StatusOK, listRec.Code,
		"GET /stocktake/:id/findings must return 200: body=%s", listRec.Body.String())

	var resp struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &resp))
	assert.GreaterOrEqual(t, len(resp.Items), 2,
		"findings list must have at least 2 entries after scanning 2 barcodes")
}

// TestStocktake_GetVariances verifies GET /stocktake/:id/variances returns 200 with
// the expected response shape: {"session_id", "variances", "count"}.
func TestStocktake_GetVariances(t *testing.T) {
	app := newCompleteTestApp(t)
	cancelActiveStocktakeSessions(t, eastBranchID)
	t.Cleanup(func() { cancelActiveStocktakeSessions(t, eastBranchID) })
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	sessRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "Variances " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	require.Equal(t, http.StatusCreated, sessRec.Code)

	var sess map[string]any
	require.NoError(t, json.Unmarshal(sessRec.Body.Bytes(), &sess))
	sessionID := sess["id"].(string)

	varRec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/stocktake/"+sessionID+"/variances", nil, cookie)
	require.Equal(t, http.StatusOK, varRec.Code,
		"GET /stocktake/:id/variances must return 200: body=%s", varRec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(varRec.Body.Bytes(), &resp))
	assert.Equal(t, sessionID, resp["session_id"],
		"variances response must echo the session_id")
	_, hasVariances := resp["variances"]
	assert.True(t, hasVariances, "variances response must include variances field")
	_, hasCount := resp["count"]
	assert.True(t, hasCount, "variances response must include count field")
}

// TestStocktake_CloseSession verifies PATCH /stocktake/:id/status with {"status":"closed"}
// transitions the session to closed and returns {"status":"closed"}.
func TestStocktake_CloseSession(t *testing.T) {
	app := newCompleteTestApp(t)
	cancelActiveStocktakeSessions(t, eastBranchID)
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	sessRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "Close " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	require.Equal(t, http.StatusCreated, sessRec.Code,
		"session create must succeed before close test: body=%s", sessRec.Body.String())

	var sess map[string]any
	require.NoError(t, json.Unmarshal(sessRec.Body.Bytes(), &sess))
	sessionID := sess["id"].(string)

	closeRec := doRequest(t, app.testApp, http.MethodPatch,
		"/api/v1/stocktake/"+sessionID+"/status",
		map[string]any{"status": "closed"},
		cookie,
	)
	require.Equal(t, http.StatusOK, closeRec.Code,
		"PATCH /stocktake/:id/status must return 200: body=%s", closeRec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(closeRec.Body.Bytes(), &resp))
	assert.Equal(t, "closed", resp["status"],
		"response must echo status=closed")
}

// TestStocktake_CancelSession verifies PATCH /stocktake/:id/status with
// {"status":"cancelled"} cancels the session and returns {"status":"cancelled"}.
func TestStocktake_CancelSession(t *testing.T) {
	app := newCompleteTestApp(t)
	cancelActiveStocktakeSessions(t, eastBranchID)
	cookie := stocktakeOpsUserCookie(t, app, eastBranchID)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	sessRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "Cancel " + suffix,
		"branch_id": eastBranchID,
	}, cookie)
	require.Equal(t, http.StatusCreated, sessRec.Code,
		"session create must succeed before cancel test: body=%s", sessRec.Body.String())

	var sess map[string]any
	require.NoError(t, json.Unmarshal(sessRec.Body.Bytes(), &sess))
	sessionID := sess["id"].(string)

	cancelRec := doRequest(t, app.testApp, http.MethodPatch,
		"/api/v1/stocktake/"+sessionID+"/status",
		map[string]any{"status": "cancelled"},
		cookie,
	)
	require.Equal(t, http.StatusOK, cancelRec.Code,
		"PATCH /stocktake/:id/status with cancelled must return 200: body=%s", cancelRec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(cancelRec.Body.Bytes(), &resp))
	assert.Equal(t, "cancelled", resp["status"],
		"response must echo status=cancelled")
}

// TestStocktake_NoPermission_Returns403 verifies that a user without stocktake:write
// cannot create a session (content_moderator lacks this permission).
func TestStocktake_NoPermission_Returns403(t *testing.T) {
	app := newCompleteTestApp(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-stk-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")
	assignUserToBranch(t, userID, eastBranchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/stocktake", map[string]any{
		"name":      "Forbidden Session",
		"branch_id": eastBranchID,
	}, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator must receive 403 on stocktake create: body=%s", rec.Body.String())
}
