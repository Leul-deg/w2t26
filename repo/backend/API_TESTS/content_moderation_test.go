package apitests

// content_moderation_test.go exercises the full content + moderation workflow
// over real HTTP:
//
//   POST /api/v1/content              (create draft)
//   GET  /api/v1/content              (list)
//   GET  /api/v1/content/:id          (get)
//   PATCH /api/v1/content/:id         (update draft)
//   POST /api/v1/content/:id/submit   (submit for review → pending_review)
//   POST /api/v1/content/:id/retract  (retract → draft)
//   POST /api/v1/content/:id/publish  (publish approved content)
//   POST /api/v1/content/:id/archive  (archive published content)
//
//   GET  /api/v1/moderation/queue              (list pending items)
//   GET  /api/v1/moderation/items/:id          (get item + content)
//   POST /api/v1/moderation/items/:id/assign   (assign to self)
//   POST /api/v1/moderation/items/:id/decide   (approve / reject)

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// contentModUserCookie creates a content_moderator user scoped to mainBranchID.
// content_moderator has: content:read, content:submit, content:moderate,
// content:publish, feedback:read, feedback:moderate, appeals:read, appeals:decide.
func contentModUserCookie(t *testing.T, app *completeTestApp) *http.Cookie {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "cmod-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")
	assignUserToBranch(t, userID, mainBranchID)
	return loginAs(t, app.testApp, username, "Password123!")
}

// ── Content CRUD tests ────────────────────────────────────────────────────────

// TestContent_CreateAndGet verifies POST /content creates a draft (201) and
// GET /content/:id retrieves it with status "draft".
func TestContent_CreateAndGet(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	title := "API Test Doc " + suffix

	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/content", map[string]any{
		"title":        title,
		"content_type": "announcement",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code,
		"POST /content must return 201: body=%s", createRec.Body.String())

	var cr map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &cr))
	contentID, ok := cr["id"].(string)
	require.True(t, ok && contentID != "", "response must include non-empty id")
	assert.Equal(t, "draft", cr["status"], "newly created content must have status=draft")

	// Get.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/content/"+contentID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code,
		"GET /content/:id must return 200: body=%s", getRec.Body.String())

	var gr map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &gr))
	assert.Equal(t, contentID, gr["id"])
	assert.Equal(t, title, gr["title"])
	assert.Equal(t, "draft", gr["status"])
}

// TestContent_List verifies GET /content returns 200 with a paginated list.
func TestContent_List(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/content", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /content must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, hasItems := resp["items"]
	assert.True(t, hasItems, "list response must include items field")
}

// TestContent_Update verifies PATCH /content/:id updates title while item is in draft.
func TestContent_Update(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/content", map[string]any{
		"title":        "Original " + suffix,
		"content_type": "document",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var cr map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &cr))
	contentID := cr["id"].(string)

	updatedTitle := "Updated " + suffix
	patchRec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/content/"+contentID, map[string]any{
		"title": updatedTitle,
	}, cookie)
	require.Equal(t, http.StatusOK, patchRec.Code,
		"PATCH /content/:id must return 200: body=%s", patchRec.Body.String())

	var pr map[string]any
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &pr))
	assert.Equal(t, updatedTitle, pr["title"], "patched title must match new value")
}

// TestContent_GetUnknown_Returns404 verifies that GET /content/:id with a
// non-existent UUID returns 404.
func TestContent_GetUnknown_Returns404(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/content/00000000-0000-0000-0000-000000000000", nil, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"unknown content id must return 404: body=%s", rec.Body.String())
}

// TestContent_NoPermission_Returns403 verifies that a user without content:submit
// receives 403 when attempting to create content.
func TestContent_NoPermission_Returns403(t *testing.T) {
	app := newCompleteTestApp(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-content-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, mainBranchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/content", map[string]any{
		"title":        "Forbidden",
		"content_type": "announcement",
	}, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"operations_staff must get 403 on content create (lacks content:submit): body=%s", rec.Body.String())
}

// ── Content lifecycle: submit → moderation → publish ────────────────────────

// TestContent_FullLifecycle_SubmitApprovePublish exercises the critical path:
// create → submit → moderator assigns+approves → publish → archive.
func TestContent_FullLifecycle_SubmitApprovePublish(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// 1. Create draft.
	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/content", map[string]any{
		"title":        "Lifecycle " + suffix,
		"content_type": "policy",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code,
		"create draft must return 201: body=%s", createRec.Body.String())

	var cr map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &cr))
	contentID := cr["id"].(string)

	// 2. Submit → pending_review (also creates a moderation item).
	submitRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/content/"+contentID+"/submit", nil, cookie)
	require.Equal(t, http.StatusOK, submitRec.Code,
		"POST /content/:id/submit must return 200: body=%s", submitRec.Body.String())

	var submitted map[string]any
	require.NoError(t, json.Unmarshal(submitRec.Body.Bytes(), &submitted))
	assert.Equal(t, "pending_review", submitted["status"],
		"submitted content must have status=pending_review")

	// 3. List moderation queue — our item must appear.
	queueRec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/moderation/queue", nil, cookie)
	require.Equal(t, http.StatusOK, queueRec.Code,
		"GET /moderation/queue must return 200: body=%s", queueRec.Body.String())

	var queue struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(queueRec.Body.Bytes(), &queue))

	// Find the moderation item for our content.
	var modItemID string
	for _, item := range queue.Items {
		if item["content_id"] == contentID {
			modItemID, _ = item["id"].(string)
			break
		}
	}
	require.NotEmpty(t, modItemID,
		"submitted content must appear in the moderation queue")

	// 4. Get the moderation item — must have item + content fields.
	getItemRec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/moderation/items/"+modItemID, nil, cookie)
	require.Equal(t, http.StatusOK, getItemRec.Code,
		"GET /moderation/items/:id must return 200: body=%s", getItemRec.Body.String())

	var itemResp map[string]any
	require.NoError(t, json.Unmarshal(getItemRec.Body.Bytes(), &itemResp))
	_, hasItem := itemResp["item"]
	assert.True(t, hasItem, "moderation item response must have item field")
	_, hasContent := itemResp["content"]
	assert.True(t, hasContent, "moderation item response must have content field")

	// 5. Assign moderation item to self.
	assignRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/moderation/items/"+modItemID+"/assign", nil, cookie)
	require.Equal(t, http.StatusOK, assignRec.Code,
		"POST /moderation/items/:id/assign must return 200: body=%s", assignRec.Body.String())

	// 6. Decide: approve.
	decideRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/moderation/items/"+modItemID+"/decide",
		map[string]any{"decision": "approved", "reason": "Looks good"},
		cookie,
	)
	require.Equal(t, http.StatusOK, decideRec.Code,
		"POST /moderation/items/:id/decide must return 200: body=%s", decideRec.Body.String())

	var decided map[string]any
	require.NoError(t, json.Unmarshal(decideRec.Body.Bytes(), &decided))
	assert.Equal(t, "decided", decided["status"],
		"decided moderation item must have status=decided")
	assert.Equal(t, "approved", decided["decision"],
		"approved moderation item must have decision=approved")

	// 7. Publish the approved content.
	publishRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/content/"+contentID+"/publish", nil, cookie)
	require.Equal(t, http.StatusOK, publishRec.Code,
		"POST /content/:id/publish must return 200: body=%s", publishRec.Body.String())

	var published map[string]any
	require.NoError(t, json.Unmarshal(publishRec.Body.Bytes(), &published))
	assert.Equal(t, "published", published["status"],
		"published content must have status=published")

	// 8. Archive the published content.
	archiveRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/content/"+contentID+"/archive", nil, cookie)
	require.Equal(t, http.StatusOK, archiveRec.Code,
		"POST /content/:id/archive must return 200: body=%s", archiveRec.Body.String())

	var archived map[string]any
	require.NoError(t, json.Unmarshal(archiveRec.Body.Bytes(), &archived))
	assert.Equal(t, "archived", archived["status"],
		"archived content must have status=archived")
}

// TestContent_Retract_Returns200 verifies that a submitted (pending_review) item
// can be retracted back to draft.
func TestContent_Retract_Returns200(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/content", map[string]any{
		"title":        "Retract Me " + suffix,
		"content_type": "announcement",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var cr map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &cr))
	contentID := cr["id"].(string)

	// Submit.
	submitRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/content/"+contentID+"/submit", nil, cookie)
	require.Equal(t, http.StatusOK, submitRec.Code,
		"submit before retract test must succeed: body=%s", submitRec.Body.String())

	// Retract.
	retractRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/content/"+contentID+"/retract", nil, cookie)
	require.Equal(t, http.StatusOK, retractRec.Code,
		"POST /content/:id/retract must return 200: body=%s", retractRec.Body.String())

	var retracted map[string]any
	require.NoError(t, json.Unmarshal(retractRec.Body.Bytes(), &retracted))
	assert.Equal(t, "draft", retracted["status"],
		"retracted content must return to status=draft")
}

// TestContent_Reject_BlocksPublish verifies that deciding to reject a moderation
// item prevents the content from being published.
func TestContent_Reject_BlocksPublish(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModUserCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/content", map[string]any{
		"title":        "Reject Me " + suffix,
		"content_type": "document",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var cr map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &cr))
	contentID := cr["id"].(string)

	// Submit.
	doRequest(t, app.testApp, http.MethodPost, "/api/v1/content/"+contentID+"/submit", nil, cookie)

	// Get the moderation item.
	queueRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/moderation/queue", nil, cookie)
	require.Equal(t, http.StatusOK, queueRec.Code)

	var queue struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(queueRec.Body.Bytes(), &queue))

	var modItemID string
	for _, item := range queue.Items {
		if item["content_id"] == contentID {
			modItemID, _ = item["id"].(string)
			break
		}
	}
	require.NotEmpty(t, modItemID)

	// Decide: reject.
	doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/moderation/items/"+modItemID+"/decide",
		map[string]any{"decision": "rejected", "reason": "Not appropriate"},
		cookie,
	)

	// Attempt to publish rejected content — must not return 200.
	publishRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/content/"+contentID+"/publish", nil, cookie)
	assert.NotEqual(t, http.StatusOK, publishRec.Code,
		"publishing rejected content must not return 200: body=%s", publishRec.Body.String())
}
