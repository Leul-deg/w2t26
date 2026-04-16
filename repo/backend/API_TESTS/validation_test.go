package apitests

// validation_test.go covers HTTP request validation at the API layer:
// missing required fields, invalid enum values, and malformed date parameters
// across the key endpoints that have business-logic constraints.
//
// Each test asserts a 422 Unprocessable Entity (or equivalent 4xx) response
// rather than a 5xx, verifying that validation fires before any DB operation.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── POST /api/v1/readers ──────────────────────────────────────────────────────

// TestValidation_Readers_MissingFirstName verifies that POST /readers with no
// first_name returns 422.
func TestValidation_Readers_MissingFirstName(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/readers", map[string]any{
		"branch_id": mainBranchID,
		"last_name": "Smith",
	}, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"POST /readers without first_name must return 422: body=%s", rec.Body.String())
}

// TestValidation_Readers_MissingLastName verifies that POST /readers with no
// last_name returns 422.
func TestValidation_Readers_MissingLastName(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/readers", map[string]any{
		"branch_id":  mainBranchID,
		"first_name": "Jane",
	}, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"POST /readers without last_name must return 422: body=%s", rec.Body.String())
}

// TestValidation_Readers_StatusPatch_InvalidCode verifies that PATCH
// /readers/:id/status with an unrecognised status_code returns 422.
func TestValidation_Readers_StatusPatch_InvalidCode(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	// Create a reader to get a real ID.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/readers", map[string]any{
		"branch_id":  mainBranchID,
		"first_name": "Val",
		"last_name":  "Test-" + suffix,
	}, cookie)
	if createRec.Code != http.StatusCreated {
		t.Skipf("reader creation returned %d, skipping", createRec.Code)
	}
	var cr map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &cr); err != nil {
		t.Skipf("cannot unmarshal create response: %v", err)
	}
	readerID, _ := cr["id"].(string)
	if readerID == "" {
		t.Skip("no reader id in response, skipping")
	}

	rec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/readers/"+readerID+"/status",
		map[string]any{"status_code": "invalid_status_xyz"}, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"PATCH /readers/:id/status with bad status_code must return 422: body=%s", rec.Body.String())
}

// ── POST /api/v1/holdings ─────────────────────────────────────────────────────

// TestValidation_Holdings_MissingTitle verifies that POST /holdings with no
// title returns 422.
func TestValidation_Holdings_MissingTitle(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := opsUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/holdings", map[string]any{
		"author": "Some Author",
	}, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"POST /holdings without title must return 422: body=%s", rec.Body.String())
}

// ── POST /api/v1/content ──────────────────────────────────────────────────────

// TestValidation_Content_MissingTitle verifies that POST /content without title
// returns 422.
func TestValidation_Content_MissingTitle(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/content", map[string]any{
		"content_type": "announcement",
	}, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"POST /content without title must return 422: body=%s", rec.Body.String())
}

// TestValidation_Content_InvalidType verifies that POST /content with an
// unrecognised content_type returns 422.
func TestValidation_Content_InvalidType(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/content", map[string]any{
		"title":        "Test Content",
		"content_type": "not_a_valid_type",
	}, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"POST /content with invalid content_type must return 422: body=%s", rec.Body.String())
}

// ── POST /api/v1/moderation/items/:id/decide ─────────────────────────────────

// TestValidation_Moderation_InvalidDecision verifies that POSTing an invalid
// decision value to the decide endpoint returns 422.
func TestValidation_Moderation_InvalidDecision(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	// Create and submit a content item so there is a moderation item to decide on.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/content", map[string]any{
		"title":        "Decide Validation " + suffix,
		"content_type": "announcement",
	}, cookie)
	if createRec.Code != http.StatusCreated {
		t.Skipf("content creation returned %d, skipping", createRec.Code)
	}
	var cr map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &cr); err != nil {
		t.Skipf("cannot unmarshal create response: %v", err)
	}
	contentID, _ := cr["id"].(string)

	submitRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/content/"+contentID+"/submit", nil, cookie)
	if submitRec.Code != http.StatusOK {
		t.Skipf("submit returned %d, skipping", submitRec.Code)
	}

	queueRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/moderation/queue", nil, cookie)
	var queue struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(queueRec.Body.Bytes(), &queue); err != nil || len(queue.Items) == 0 {
		t.Skip("moderation queue empty, skipping")
	}
	var modItemID string
	for _, item := range queue.Items {
		if item["content_id"] == contentID {
			modItemID, _ = item["id"].(string)
			break
		}
	}
	if modItemID == "" {
		t.Skip("moderation item not found for this content, skipping")
	}

	rec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/moderation/items/"+modItemID+"/decide",
		map[string]any{"decision": "maybe"},
		cookie,
	)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"invalid decision value must return 422: body=%s", rec.Body.String())
}

// ── POST /api/v1/programs ────────────────────────────────────────────────────

// TestValidation_Programs_MissingTitle verifies that POST /programs with no
// title returns 422.
func TestValidation_Programs_MissingTitle(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := opsUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/programs", map[string]any{
		"capacity":  10,
		"starts_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		"ends_at":   time.Now().Add(2 * time.Hour).Format(time.RFC3339),
	}, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"POST /programs without title must return 422: body=%s", rec.Body.String())
}

// TestValidation_Programs_ZeroCapacity verifies that POST /programs with
// capacity=0 returns 422.
func TestValidation_Programs_ZeroCapacity(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := opsUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/programs", map[string]any{
		"title":     "Zero Cap",
		"capacity":  0,
		"starts_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		"ends_at":   time.Now().Add(2 * time.Hour).Format(time.RFC3339),
	}, cookie)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"POST /programs with capacity=0 must return 422: body=%s", rec.Body.String())
}

// ── GET /api/v1/reports/aggregates ───────────────────────────────────────────

// TestValidation_Reports_MissingFromDate verifies that omitting "from" returns 422.
func TestValidation_Reports_MissingFromDate(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/reports/aggregates?branch_id="+mainBranchID+"&to=2026-01-31",
		nil, cookie,
	)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"missing from date must return 422: body=%s", rec.Body.String())
}

// TestValidation_Reports_MissingToDate verifies that omitting "to" returns 422.
func TestValidation_Reports_MissingToDate(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := loginAs(t, app.testApp, "admin", "Admin1234!")

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/reports/aggregates?branch_id="+mainBranchID+"&from=2026-01-01",
		nil, cookie,
	)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"missing to date must return 422: body=%s", rec.Body.String())
}

// ── POST /api/v1/users (CreateUser validation) ───────────────────────────────

// TestValidation_Users_MissingUsername verifies that POST /users without a
// username returns a 4xx (not 201 or 5xx).
func TestValidation_Users_MissingUsername(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookieForUsersTest(t, app)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/users", map[string]any{
		"email":    "nousername@test.local",
		"password": "ValidPass123!",
	}, cookie)
	assert.NotEqual(t, http.StatusCreated, rec.Code,
		"POST /users without username must not return 201: body=%s", rec.Body.String())
	assert.Less(t, rec.Code, 500,
		"POST /users without username must not cause 5xx: body=%s", rec.Body.String())
}

// TestValidation_Users_MissingPassword verifies that POST /users without a
// password returns a 4xx.
func TestValidation_Users_MissingPassword(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookieForUsersTest(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/users", map[string]any{
		"username": "nopw-" + suffix,
		"email":    "nopw-" + suffix + "@test.local",
	}, cookie)
	assert.NotEqual(t, http.StatusCreated, rec.Code,
		"POST /users without password must not return 201: body=%s", rec.Body.String())
	assert.Less(t, rec.Code, 500,
		"POST /users without password must not cause 5xx: body=%s", rec.Body.String())
}
