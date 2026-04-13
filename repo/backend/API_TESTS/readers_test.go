package apitests

// readers_test.go tests the full readers CRUD API:
//   GET  /api/v1/readers/statuses
//   GET  /api/v1/readers
//   POST /api/v1/readers
//   GET  /api/v1/readers/:id
//   PATCH /api/v1/readers/:id
//   PATCH /api/v1/readers/:id/status
//   GET  /api/v1/readers/:id/history
//   GET  /api/v1/readers/:id/holdings

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

const mainBranchID = "bbbbbbbb-0000-0000-0000-000000000001"

// adminCookie logs in as the seeded admin and returns the session cookie.
func adminCookie(t *testing.T, app *completeTestApp) *http.Cookie {
	t.Helper()
	return loginAs(t, app.testApp, "admin", "Admin1234!")
}

// TestReaders_ListStatuses verifies GET /api/v1/readers/statuses returns
// the seeded statuses (active, suspended, expired, etc.).
func TestReaders_ListStatuses(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/readers/statuses", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /readers/statuses must return 200: body=%s", rec.Body.String())

	var statuses []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &statuses))
	assert.NotEmpty(t, statuses, "at least one reader status must be seeded")

	// Verify 'active' is present.
	var found bool
	for _, s := range statuses {
		if s["code"] == "active" {
			found = true
		}
	}
	assert.True(t, found, "reader status 'active' must be in the seeded list")
}

// TestReaders_List_AdminSeesAll verifies that admin (empty branch scope) gets 200
// with a paginated response envelope.
func TestReaders_List_AdminSeesAll(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/readers?page=1&per_page=10", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"admin GET /readers must return 200: body=%s", rec.Body.String())

	var resp struct {
		Items   []map[string]any `json:"items"`
		Page    int              `json:"page"`
		PerPage int              `json:"per_page"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 10, resp.PerPage)
}

// TestReaders_CreateAndGet tests the full create → get flow:
//  1. POST /api/v1/readers creates a reader and returns 201 with the reader body.
//  2. GET /api/v1/readers/:id retrieves the same reader by the returned ID.
func TestReaders_CreateAndGet(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerNum := "API-RDR-" + suffix

	// 1. Create reader.
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/readers", map[string]any{
		"branch_id":     mainBranchID,
		"reader_number": readerNum,
		"first_name":    "Alice",
		"last_name":     "Test",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code,
		"POST /readers must return 201: body=%s", createRec.Body.String())

	var createResp map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &createResp))
	readerID, ok := createResp["id"].(string)
	require.True(t, ok && readerID != "", "response must include non-empty id")

	t.Cleanup(func() {
		pool := testdb.Open(t)
		defer pool.Close()
		execIgnoreErr(pool, `DELETE FROM lms.readers WHERE id = $1`, readerID)
	})

	// 2. Get reader by ID.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/readers/"+readerID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code,
		"GET /readers/:id must return 200: body=%s", getRec.Body.String())

	var getResp map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	assert.Equal(t, readerID, getResp["id"], "returned reader ID must match")
	assert.Equal(t, "Alice", getResp["first_name"])
	assert.Equal(t, "Test", getResp["last_name"])
	assert.Equal(t, readerNum, getResp["reader_number"])
}

// TestReaders_Update verifies PATCH /api/v1/readers/:id updates name fields.
func TestReaders_Update(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "UPD-"+suffix)

	rec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/readers/"+readerID, map[string]any{
		"first_name": "Updated",
		"last_name":  "Name",
	}, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"PATCH /readers/:id must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Updated", resp["first_name"], "first_name must be updated")
	assert.Equal(t, "Name", resp["last_name"], "last_name must be updated")
}

// TestReaders_UpdateStatus verifies PATCH /api/v1/readers/:id/status changes status.
func TestReaders_UpdateStatus(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "STS-"+suffix)

	rec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/readers/"+readerID+"/status",
		map[string]any{"status_code": "suspended"},
		cookie,
	)
	require.Equal(t, http.StatusOK, rec.Code,
		"PATCH /readers/:id/status must return 200: body=%s", rec.Body.String())

	// Confirm status by fetching the reader.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/readers/"+readerID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code)
	var rdr map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &rdr))
	assert.Equal(t, "suspended", rdr["status_code"], "reader status_code must be suspended after update")
}

// TestReaders_GetLoanHistory verifies GET /api/v1/readers/:id/history returns 200
// with a paginated response (even if empty).
func TestReaders_GetLoanHistory(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "HIST-"+suffix)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/readers/"+readerID+"/history", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /readers/:id/history must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, hasItems := resp["items"]
	assert.True(t, hasItems, "loan history response must have items array")
}

// TestReaders_GetCurrentHoldings verifies GET /api/v1/readers/:id/holdings returns 200.
func TestReaders_GetCurrentHoldings(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "HOLD-"+suffix)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/readers/"+readerID+"/holdings", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /readers/:id/holdings must return 200: body=%s", rec.Body.String())
}

// TestReaders_GetUnknownID_Returns404 verifies that requesting a non-existent
// reader ID returns 404 (not 500).
func TestReaders_GetUnknownID_Returns404(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/readers/00000000-dead-beef-0000-000000000000", nil, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"unknown reader ID must return 404: body=%s", rec.Body.String())
}

// TestReaders_Create_DuplicateReaderNumber_Returns409 verifies that inserting
// two readers with the same reader_number returns a conflict error.
func TestReaders_Create_DuplicateReaderNumber_Returns409(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerNum := "DUP-" + suffix

	// First insert — must succeed.
	rec1 := doRequest(t, app.testApp, http.MethodPost, "/api/v1/readers", map[string]any{
		"branch_id": mainBranchID, "reader_number": readerNum,
		"first_name": "First", "last_name": "Reader",
	}, cookie)
	require.Equal(t, http.StatusCreated, rec1.Code,
		"first reader create must return 201: body=%s", rec1.Body.String())

	var r1 map[string]any
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &r1))
	t.Cleanup(func() {
		pool := testdb.Open(t)
		defer pool.Close()
		execIgnoreErr(pool, `DELETE FROM lms.readers WHERE id = $1`, r1["id"])
	})

	// Second insert with same reader_number — must fail with 409.
	rec2 := doRequest(t, app.testApp, http.MethodPost, "/api/v1/readers", map[string]any{
		"branch_id": mainBranchID, "reader_number": readerNum,
		"first_name": "Second", "last_name": "Reader",
	}, cookie)
	assert.Equal(t, http.StatusConflict, rec2.Code,
		"duplicate reader_number must return 409: body=%s", rec2.Body.String())
}

// TestReaders_ListWithSearch verifies that the search query param filters results.
func TestReaders_ListWithSearch(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	uniqueLastName := "Zxqvbn" + suffix
	insertAPITestReaderNamed(t, mainBranchID, "SRH-"+suffix, "Search", uniqueLastName)

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/readers?search="+uniqueLastName, nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /readers?search must return 200: body=%s", rec.Body.String())

	var resp struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Items, "search for unique last name must return at least 1 result")

	found := false
	for _, item := range resp.Items {
		if item["last_name"] == uniqueLastName {
			found = true
		}
	}
	assert.True(t, found, "searched reader must appear in results")
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// insertAPITestReader inserts a minimal reader into the given branch and registers
// cleanup. Returns the reader UUID string.
func insertAPITestReader(t *testing.T, branchID, readerNumber string) string {
	t.Helper()
	return insertAPITestReaderNamed(t, branchID, readerNumber, "API", "Test")
}

func insertAPITestReaderNamed(t *testing.T, branchID, readerNumber, firstName, lastName string) string {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()

	var id string
	err := pool.QueryRow(contextBG(),
		`INSERT INTO lms.readers (branch_id, reader_number, first_name, last_name, status_code)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id::text`,
		branchID, readerNumber, firstName, lastName,
	).Scan(&id)
	require.NoError(t, err, "insertAPITestReader: insert failed")

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		execIgnoreErr(p, `DELETE FROM lms.readers WHERE id = $1`, id)
	})
	return id
}
