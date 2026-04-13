package apitests

// holdings_copies_test.go tests the full holdings + copies CRUD API:
//   GET  /api/v1/holdings
//   POST /api/v1/holdings
//   GET  /api/v1/holdings/:id
//   PATCH /api/v1/holdings/:id
//   DELETE /api/v1/holdings/:id
//   GET  /api/v1/holdings/:id/copies
//   POST /api/v1/holdings/:id/copies
//   GET  /api/v1/copies/statuses
//   GET  /api/v1/copies/lookup?barcode=...
//   GET  /api/v1/copies/:id
//   PATCH /api/v1/copies/:id
//   PATCH /api/v1/copies/:id/status

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

// TestHoldings_CreateAndGet verifies POST /holdings creates a holding (201) and
// GET /holdings/:id retrieves it (200) with correct fields.
func TestHoldings_CreateAndGet(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	title := "API Test Book " + suffix

	// Create.
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/holdings", map[string]any{
		"branch_id": mainBranchID,
		"title":     title,
		"language":  "en",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code,
		"POST /holdings must return 201: body=%s", createRec.Body.String())

	var cr map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &cr))
	holdingID, ok := cr["id"].(string)
	require.True(t, ok && holdingID != "", "response must include non-empty id")

	t.Cleanup(func() {
		pool := testdb.Open(t)
		defer pool.Close()
		execIgnoreErr(pool, `DELETE FROM lms.copies WHERE holding_id = $1`, holdingID)
		execIgnoreErr(pool, `DELETE FROM lms.holdings WHERE id = $1`, holdingID)
	})

	// Get.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/holdings/"+holdingID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code,
		"GET /holdings/:id must return 200: body=%s", getRec.Body.String())

	var gr map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &gr))
	assert.Equal(t, holdingID, gr["id"])
	assert.Equal(t, title, gr["title"])
}

// TestHoldings_List verifies GET /holdings returns a paginated 200 response.
func TestHoldings_List(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/holdings?page=1&per_page=5", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /holdings must return 200: body=%s", rec.Body.String())

	var resp struct {
		Items   []map[string]any `json:"items"`
		Page    int              `json:"page"`
		PerPage int              `json:"per_page"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 5, resp.PerPage)
}

// TestHoldings_Update verifies PATCH /holdings/:id updates fields and returns 200.
func TestHoldings_Update(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	holdingID := insertAPITestHolding(t, mainBranchID)

	rec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/holdings/"+holdingID, map[string]any{
		"title":    "Updated Title",
		"language": "fr",
	}, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"PATCH /holdings/:id must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Updated Title", resp["title"])
}

// TestHoldings_Deactivate verifies DELETE /holdings/:id soft-deletes and returns 204.
func TestHoldings_Deactivate(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	holdingID := insertAPITestHolding(t, mainBranchID)

	rec := doRequest(t, app.testApp, http.MethodDelete, "/api/v1/holdings/"+holdingID, nil, cookie)
	assert.Equal(t, http.StatusNoContent, rec.Code,
		"DELETE /holdings/:id must return 204: body=%s", rec.Body.String())
}

// TestHoldings_GetUnknown_Returns404 verifies that GET /holdings/:id with a
// non-existent UUID returns 404.
func TestHoldings_GetUnknown_Returns404(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/holdings/00000000-0000-0000-0000-000000000000", nil, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"unknown holding ID must return 404: body=%s", rec.Body.String())
}

// ── Copies ────────────────────────────────────────────────────────────────────

// TestCopies_Statuses verifies GET /copies/statuses returns the seeded statuses.
func TestCopies_Statuses(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/copies/statuses", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /copies/statuses must return 200: body=%s", rec.Body.String())

	var statuses []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &statuses))
	assert.NotEmpty(t, statuses, "at least one copy status must be seeded")

	// 'available' must be present.
	found := false
	for _, s := range statuses {
		if s["code"] == "available" {
			found = true
		}
	}
	assert.True(t, found, "copy status 'available' must be seeded")
}

// TestCopies_AddAndGet verifies POST /holdings/:id/copies adds a copy (201) and
// GET /copies/:id retrieves it (200).
func TestCopies_AddAndGet(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	holdingID := insertAPITestHolding(t, mainBranchID)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	barcode := "COPY-" + suffix

	// Add copy.
	addRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/holdings/"+holdingID+"/copies", map[string]any{
		"barcode":     barcode,
		"status_code": "available",
		"condition":   "good",
	}, cookie)
	require.Equal(t, http.StatusCreated, addRec.Code,
		"POST /holdings/:id/copies must return 201: body=%s", addRec.Body.String())

	var ar map[string]any
	require.NoError(t, json.Unmarshal(addRec.Body.Bytes(), &ar))
	copyID, ok := ar["id"].(string)
	require.True(t, ok && copyID != "", "response must include non-empty id")

	t.Cleanup(func() {
		pool := testdb.Open(t)
		defer pool.Close()
		execIgnoreErr(pool, `DELETE FROM lms.copies WHERE id = $1`, copyID)
	})

	// Get copy.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/copies/"+copyID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code,
		"GET /copies/:id must return 200: body=%s", getRec.Body.String())

	var gr map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &gr))
	assert.Equal(t, copyID, gr["id"])
	assert.Equal(t, barcode, gr["barcode"])
	assert.Equal(t, "available", gr["status_code"])
}

// TestCopies_ListForHolding verifies GET /holdings/:id/copies returns copies list.
func TestCopies_ListForHolding(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	holdingID := insertAPITestHolding(t, mainBranchID)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	barcode := "LIST-CP-" + suffix

	// Add one copy first.
	pool := testdb.Open(t)
	var copyID string
	err := pool.QueryRow(contextBG(),
		`INSERT INTO lms.copies (holding_id, branch_id, barcode, status_code)
		 VALUES ($1, $2, $3, 'available') RETURNING id::text`,
		holdingID, mainBranchID, barcode,
	).Scan(&copyID)
	pool.Close()
	require.NoError(t, err)
	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		execIgnoreErr(p, `DELETE FROM lms.copies WHERE id = $1`, copyID)
	})

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/holdings/"+holdingID+"/copies", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /holdings/:id/copies must return 200: body=%s", rec.Body.String())

	var resp struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Items, "copies list must not be empty after adding a copy")
}

// TestCopies_LookupByBarcode verifies GET /copies/lookup?barcode=... returns 200
// for a known barcode and 404 for an unknown one.
func TestCopies_LookupByBarcode(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	holdingID := insertAPITestHolding(t, mainBranchID)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	barcode := "LKP-" + suffix

	pool := testdb.Open(t)
	var copyID string
	err := pool.QueryRow(contextBG(),
		`INSERT INTO lms.copies (holding_id, branch_id, barcode, status_code)
		 VALUES ($1, $2, $3, 'available') RETURNING id::text`,
		holdingID, mainBranchID, barcode,
	).Scan(&copyID)
	pool.Close()
	require.NoError(t, err)
	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		execIgnoreErr(p, `DELETE FROM lms.copies WHERE id = $1`, copyID)
	})

	// Known barcode — admin has no branch scope so we pass branch_id param.
	recFound := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/copies/lookup?barcode="+barcode, nil, cookie)
	require.Equal(t, http.StatusOK, recFound.Code,
		"GET /copies/lookup?barcode must return 200 for known barcode: body=%s", recFound.Body.String())

	var cp map[string]any
	require.NoError(t, json.Unmarshal(recFound.Body.Bytes(), &cp))
	assert.Equal(t, barcode, cp["barcode"])

	// Unknown barcode.
	recMissing := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/copies/lookup?barcode=UNKNOWN-99999", nil, cookie)
	assert.Equal(t, http.StatusNotFound, recMissing.Code,
		"GET /copies/lookup for unknown barcode must return 404: body=%s", recMissing.Body.String())
}

// TestCopies_UpdateStatus verifies PATCH /copies/:id/status changes the status.
func TestCopies_UpdateStatus(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	holdingID := insertAPITestHolding(t, mainBranchID)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	copyID := insertAPITestCopy(t, holdingID, mainBranchID, "UPD-STS-"+suffix)

	rec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/copies/"+copyID+"/status",
		map[string]any{"status_code": "on_loan"},
		cookie,
	)
	require.Equal(t, http.StatusOK, rec.Code,
		"PATCH /copies/:id/status must return 200: body=%s", rec.Body.String())

	// Verify by fetching the copy.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/copies/"+copyID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code)
	var cp map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &cp))
	assert.Equal(t, "on_loan", cp["status_code"])
}

// TestCopies_UpdateCondition verifies PATCH /copies/:id updates condition/notes.
func TestCopies_UpdateCondition(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	holdingID := insertAPITestHolding(t, mainBranchID)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	copyID := insertAPITestCopy(t, holdingID, mainBranchID, "COND-"+suffix)

	notes := "Updated condition notes"
	rec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/copies/"+copyID, map[string]any{
		"condition": "fair",
		"notes":     notes,
	}, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"PATCH /copies/:id must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "fair", resp["condition"])
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// insertAPITestHolding inserts a minimal holding and registers cleanup.
func insertAPITestHolding(t *testing.T, branchID string) string {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	var id string
	err := pool.QueryRow(contextBG(),
		`INSERT INTO lms.holdings (branch_id, title, language)
		 VALUES ($1, $2, 'en') RETURNING id::text`,
		branchID, "API Holding "+suffix,
	).Scan(&id)
	require.NoError(t, err, "insertAPITestHolding: insert failed")

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		execIgnoreErr(p, `DELETE FROM lms.copies WHERE holding_id = $1`, id)
		execIgnoreErr(p, `DELETE FROM lms.holdings WHERE id = $1`, id)
	})
	return id
}

// insertAPITestCopy inserts a copy into the given holding+branch and registers cleanup.
func insertAPITestCopy(t *testing.T, holdingID, branchID, barcode string) string {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()

	var id string
	err := pool.QueryRow(contextBG(),
		`INSERT INTO lms.copies (holding_id, branch_id, barcode, status_code)
		 VALUES ($1, $2, $3, 'available') RETURNING id::text`,
		holdingID, branchID, barcode,
	).Scan(&id)
	require.NoError(t, err, "insertAPITestCopy: insert failed")

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		execIgnoreErr(p, `DELETE FROM lms.copies WHERE id = $1`, id)
	})
	return id
}
