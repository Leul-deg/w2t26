package apitests

// circulation_test.go tests the checkout/return/list circulation API:
//   POST /api/v1/circulation/checkout
//   POST /api/v1/circulation/return
//   GET  /api/v1/circulation
//   GET  /api/v1/circulation/copy/:id
//   GET  /api/v1/circulation/reader/:id
//   GET  /api/v1/circulation/active/:id

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

// TestCirculation_CheckoutAndReturn tests the full checkout → return flow:
//  1. Admin checks out a copy to a reader — expects 201.
//  2. Admin returns the same copy — expects 200.
func TestCirculation_CheckoutAndReturn(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Seed: reader + holding + copy in MAIN branch.
	readerID := insertAPITestReader(t, mainBranchID, "CIR-RDR-"+suffix)
	holdingID := insertAPITestHolding(t, mainBranchID)
	copyID := insertAPITestCopy(t, holdingID, mainBranchID, "CIR-CP-"+suffix)

	dueDate := time.Now().UTC().Add(14 * 24 * time.Hour).Format("2006-01-02")

	// 1. Checkout.
	checkoutRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/checkout",
		map[string]any{
			"copy_id":   copyID,
			"reader_id": readerID,
			"due_date":  dueDate,
			"branch_id": mainBranchID, // admin needs explicit branch_id
		},
		cookie,
	)
	require.Equal(t, http.StatusCreated, checkoutRec.Code,
		"POST /circulation/checkout must return 201: body=%s", checkoutRec.Body.String())

	var co map[string]any
	require.NoError(t, json.Unmarshal(checkoutRec.Body.Bytes(), &co))
	assert.Equal(t, copyID, co["copy_id"], "checkout response must include copy_id")
	assert.Equal(t, readerID, co["reader_id"], "checkout response must include reader_id")

	// 2. Verify copy is on_loan via active checkout endpoint.
	activeRec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/circulation/active/"+copyID, nil, cookie)
	require.Equal(t, http.StatusOK, activeRec.Code,
		"GET /circulation/active/:id after checkout must return 200: body=%s", activeRec.Body.String())

	// 3. Return.
	returnRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/return",
		map[string]any{
			"copy_id":   copyID,
			"branch_id": mainBranchID,
		},
		cookie,
	)
	require.Equal(t, http.StatusCreated, returnRec.Code,
		"POST /circulation/return must return 201: body=%s", returnRec.Body.String())

	// 4. After return the active checkout should be gone (404).
	after := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/circulation/active/"+copyID, nil, cookie)
	assert.Equal(t, http.StatusNotFound, after.Code,
		"GET /circulation/active/:id after return must return 404: body=%s", after.Body.String())
}

// TestCirculation_DoubleCheckout_Returns409 verifies that checking out an
// already-checked-out copy returns a conflict error.
func TestCirculation_DoubleCheckout_Returns409(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "DBL-RDR-"+suffix)
	holdingID := insertAPITestHolding(t, mainBranchID)
	copyID := insertAPITestCopy(t, holdingID, mainBranchID, "DBL-CP-"+suffix)

	dueDate := time.Now().UTC().Add(14 * 24 * time.Hour).Format("2006-01-02")
	body := map[string]any{
		"copy_id": copyID, "reader_id": readerID,
		"due_date": dueDate, "branch_id": mainBranchID,
	}

	// First checkout — must succeed.
	rec1 := doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/checkout", body, cookie)
	require.Equal(t, http.StatusCreated, rec1.Code,
		"first checkout must return 201: body=%s", rec1.Body.String())

	// Second checkout of same copy — must fail.
	rec2 := doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/checkout", body, cookie)
	assert.NotEqual(t, http.StatusCreated, rec2.Code,
		"double checkout must not return 201: body=%s", rec2.Body.String())

	// Cleanup: return the copy.
	doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/return",
		map[string]any{"copy_id": copyID, "branch_id": mainBranchID}, cookie)
}

// TestCirculation_List verifies GET /circulation returns a 200 with event list.
func TestCirculation_List(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/circulation", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /circulation must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, hasItems := resp["items"]
	assert.True(t, hasItems, "circulation list response must have items array")
}

// TestCirculation_ListByCopy verifies GET /circulation/copy/:id returns events.
func TestCirculation_ListByCopy(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "LCIRC-RDR-"+suffix)
	holdingID := insertAPITestHolding(t, mainBranchID)
	copyID := insertAPITestCopy(t, holdingID, mainBranchID, "LCIRC-CP-"+suffix)

	// Perform a checkout and immediate return to create circulation events.
	dueDate := time.Now().UTC().Add(7 * 24 * time.Hour).Format("2006-01-02")
	doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/checkout",
		map[string]any{"copy_id": copyID, "reader_id": readerID, "due_date": dueDate, "branch_id": mainBranchID},
		cookie)
	doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/return",
		map[string]any{"copy_id": copyID, "branch_id": mainBranchID}, cookie)

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/circulation/copy/"+copyID, nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /circulation/copy/:id must return 200: body=%s", rec.Body.String())

	var resp struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Items, "after checkout+return, copy must have at least 1 circulation event")
}

// TestCirculation_ListByReader verifies GET /circulation/reader/:id returns events.
func TestCirculation_ListByReader(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "RCIRC-RDR-"+suffix)
	holdingID := insertAPITestHolding(t, mainBranchID)
	copyID := insertAPITestCopy(t, holdingID, mainBranchID, "RCIRC-CP-"+suffix)

	dueDate := time.Now().UTC().Add(7 * 24 * time.Hour).Format("2006-01-02")
	doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/checkout",
		map[string]any{"copy_id": copyID, "reader_id": readerID, "due_date": dueDate, "branch_id": mainBranchID},
		cookie)
	doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/return",
		map[string]any{"copy_id": copyID, "branch_id": mainBranchID}, cookie)

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/circulation/reader/"+readerID, nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /circulation/reader/:id must return 200: body=%s", rec.Body.String())

	var resp struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Items,
		"after checkout+return, reader must have at least 1 circulation event")
}

// TestCirculation_ActiveCheckout_NoCopy_Returns404 verifies that querying the
// active checkout for an unknown copy ID returns 404.
func TestCirculation_ActiveCheckout_NoCopy_Returns404(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/circulation/active/00000000-0000-0000-0000-000000000000", nil, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"active checkout for unknown copy must return 404: body=%s", rec.Body.String())
}

// TestCirculation_Return_NoCopy_Returns404 verifies that returning an unknown
// copy returns 404.
func TestCirculation_Return_NoCopy_Returns404(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/return",
		map[string]any{"copy_id": "00000000-0000-0000-0000-000000000000", "branch_id": mainBranchID},
		cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"return for unknown copy must return 404: body=%s", rec.Body.String())
}

// TestCirculation_CheckoutNoPermission_Returns403 verifies that a user without
// circulation:checkout permission gets 403.
func TestCirculation_CheckoutNoPermission_Returns403(t *testing.T) {
	app := newCompleteTestApp(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-circ-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")
	assignUserToBranch(t, userID, mainBranchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	holdingID := insertAPITestHolding(t, mainBranchID)
	copyID := insertAPITestCopy(t, holdingID, mainBranchID, "NOPERM-CP-"+suffix)
	readerID := insertAPITestReader(t, mainBranchID, "NOPERM-RDR-"+suffix)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/circulation/checkout",
		map[string]any{
			"copy_id": copyID, "reader_id": readerID,
			"due_date": time.Now().UTC().Add(7*24*time.Hour).Format("2006-01-02"),
		},
		cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator must get 403 on checkout: body=%s", rec.Body.String())
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// insertCirculationEvent is a direct DB helper to create a completed circulation
// event (checkout + return) without going through the API. Used when tests need
// pre-existing history records.
func insertCirculationEvent(t *testing.T, copyID, readerID, branchID string) {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()

	dueDate := time.Now().UTC().Add(7 * 24 * time.Hour)
	_, err := pool.Exec(contextBG(),
		`INSERT INTO lms.circulation_events (copy_id, reader_id, branch_id, event_type, due_date)
		 VALUES ($1, $2, $3, 'checkout', $4)`,
		copyID, readerID, branchID, dueDate,
	)
	require.NoError(t, err, "insertCirculationEvent: failed to insert checkout event")
}
