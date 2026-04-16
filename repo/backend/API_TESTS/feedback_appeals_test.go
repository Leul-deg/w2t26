package apitests

// feedback_appeals_test.go exercises the feedback and appeals API over real HTTP:
//
//   GET  /api/v1/feedback/tags        (list tags)
//   POST /api/v1/feedback             (submit feedback)
//   GET  /api/v1/feedback             (list feedback)
//   GET  /api/v1/feedback/:id         (get feedback)
//   POST /api/v1/feedback/:id/moderate (approve / reject / flag)
//
//   POST /api/v1/appeals              (submit appeal)
//   GET  /api/v1/appeals              (list appeals)
//   GET  /api/v1/appeals/:id          (get appeal + arbitration)
//   POST /api/v1/appeals/:id/review   (start review → under_review)
//   POST /api/v1/appeals/:id/arbitrate (issue decision)

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

// contentModCookieForFA creates a content_moderator user scoped to mainBranchID.
// This role has feedback:submit, feedback:moderate, appeals:submit, appeals:read,
// appeals:decide, and readers:read — covering all endpoints in this file.
func contentModCookieForFA(t *testing.T, app *completeTestApp) *http.Cookie {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "fa-cmod-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")
	assignUserToBranch(t, userID, mainBranchID)
	return loginAs(t, app.testApp, username, "Password123!")
}

// ── Feedback tests ────────────────────────────────────────────────────────────

// TestFeedback_ListTags verifies GET /feedback/tags returns 200 with seeded tags.
func TestFeedback_ListTags(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/feedback/tags", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /feedback/tags must return 200: body=%s", rec.Body.String())

	var tags []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tags))
	assert.NotEmpty(t, tags, "feedback tags must not be empty (seed data includes several)")
}

// TestFeedback_SubmitAndGet verifies POST /feedback creates a feedback item (201)
// and GET /feedback/:id retrieves it (200).
func TestFeedback_SubmitAndGet(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	// We need a reader and a holding to attach feedback to.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "FB-RDR-"+suffix)
	holdingID := insertAPITestHolding(t, mainBranchID)

	rating := 4
	comment := "Great resource"
	submitRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/feedback", map[string]any{
		"reader_id":   readerID,
		"target_type": "holding",
		"target_id":   holdingID,
		"rating":      rating,
		"comment":     comment,
		"tags":        []string{"helpful"},
	}, cookie)
	require.Equal(t, http.StatusCreated, submitRec.Code,
		"POST /feedback must return 201: body=%s", submitRec.Body.String())

	var fb map[string]any
	require.NoError(t, json.Unmarshal(submitRec.Body.Bytes(), &fb))
	feedbackID, ok := fb["id"].(string)
	require.True(t, ok && feedbackID != "", "response must include non-empty feedback id")
	assert.Equal(t, "pending", fb["status"],
		"newly submitted feedback must have status=pending")

	// Get.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/feedback/"+feedbackID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code,
		"GET /feedback/:id must return 200: body=%s", getRec.Body.String())

	var got map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &got))
	assert.Equal(t, feedbackID, got["id"])
}

// TestFeedback_List verifies GET /feedback returns 200 with items array.
func TestFeedback_List(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/feedback", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /feedback must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, hasItems := resp["items"]
	assert.True(t, hasItems, "feedback list response must include items field")
}

// TestFeedback_ModerateApprove verifies POST /feedback/:id/moderate with status
// "approved" transitions the feedback and returns 200.
func TestFeedback_ModerateApprove(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "FBAPP-RDR-"+suffix)
	holdingID := insertAPITestHolding(t, mainBranchID)

	submitRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/feedback", map[string]any{
		"reader_id": readerID, "target_type": "holding", "target_id": holdingID,
		"rating": 5, "comment": "Excellent",
	}, cookie)
	require.Equal(t, http.StatusCreated, submitRec.Code)

	var fb map[string]any
	require.NoError(t, json.Unmarshal(submitRec.Body.Bytes(), &fb))
	feedbackID := fb["id"].(string)

	moderateRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/feedback/"+feedbackID+"/moderate",
		map[string]any{"status": "approved"},
		cookie,
	)
	require.Equal(t, http.StatusOK, moderateRec.Code,
		"POST /feedback/:id/moderate must return 200: body=%s", moderateRec.Body.String())

	var moderated map[string]any
	require.NoError(t, json.Unmarshal(moderateRec.Body.Bytes(), &moderated))
	assert.Equal(t, "approved", moderated["status"],
		"moderated feedback must have status=approved")
}

// TestFeedback_ModerateReject verifies POST /feedback/:id/moderate with status
// "rejected" transitions the feedback to rejected.
func TestFeedback_ModerateReject(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "FBREJ-RDR-"+suffix)
	holdingID := insertAPITestHolding(t, mainBranchID)

	submitRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/feedback", map[string]any{
		"reader_id": readerID, "target_type": "holding", "target_id": holdingID,
		"comment": "Inappropriate content",
	}, cookie)
	require.Equal(t, http.StatusCreated, submitRec.Code)

	var fb map[string]any
	require.NoError(t, json.Unmarshal(submitRec.Body.Bytes(), &fb))
	feedbackID := fb["id"].(string)

	moderateRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/feedback/"+feedbackID+"/moderate",
		map[string]any{"status": "rejected"},
		cookie,
	)
	require.Equal(t, http.StatusOK, moderateRec.Code,
		"POST /feedback/:id/moderate with rejected must return 200: body=%s", moderateRec.Body.String())

	var moderated map[string]any
	require.NoError(t, json.Unmarshal(moderateRec.Body.Bytes(), &moderated))
	assert.Equal(t, "rejected", moderated["status"],
		"rejected feedback must have status=rejected")
}

// TestFeedback_NoSubmitPermission_Returns403 verifies that a user without
// feedback:submit receives 403 when posting feedback (operations_staff has this
// permission via migration 017, but an empty role does not).
func TestFeedback_NoSubmitPermission_Returns403(t *testing.T) {
	app := newCompleteTestApp(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-fb-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")
	roleID := createTempRoleWithPermissions(t, "feedback:read") // read-only, no submit
	assignUserRole(t, userID, roleID)
	assignUserToBranch(t, userID, mainBranchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/feedback", map[string]any{
		"reader_id": "00000000-0000-0000-0000-000000000099",
		"target_type": "holding",
		"target_id":   "00000000-0000-0000-0000-000000000099",
	}, cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user with only feedback:read must get 403 on submit: body=%s", rec.Body.String())
}

// ── Appeals tests ─────────────────────────────────────────────────────────────

// TestAppeals_SubmitAndGet verifies POST /appeals creates an appeal (201) and
// GET /appeals/:id returns it with arbitration field (200).
func TestAppeals_SubmitAndGet(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "APL-RDR-"+suffix)

	submitRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/appeals", map[string]any{
		"reader_id":   readerID,
		"appeal_type": "account_suspension",
		"reason":      "Unjust suspension",
	}, cookie)
	require.Equal(t, http.StatusCreated, submitRec.Code,
		"POST /appeals must return 201: body=%s", submitRec.Body.String())

	var ap map[string]any
	require.NoError(t, json.Unmarshal(submitRec.Body.Bytes(), &ap))
	appealID, ok := ap["id"].(string)
	require.True(t, ok && appealID != "", "response must include non-empty appeal id")

	// Get.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/appeals/"+appealID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code,
		"GET /appeals/:id must return 200: body=%s", getRec.Body.String())

	var got map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &got))
	_, hasAppeal := got["appeal"]
	assert.True(t, hasAppeal, "GET /appeals/:id response must include appeal field")
	_, hasArbitration := got["arbitration"]
	assert.True(t, hasArbitration, "GET /appeals/:id response must include arbitration field")
}

// TestAppeals_List verifies GET /appeals returns 200 with items array.
func TestAppeals_List(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/appeals", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /appeals must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, hasItems := resp["items"]
	assert.True(t, hasItems, "appeals list response must include items field")
}

// TestAppeals_Review_TransitionsStatus verifies POST /appeals/:id/review moves
// the appeal to under_review status.
func TestAppeals_Review_TransitionsStatus(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "APLRV-RDR-"+suffix)

	submitRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/appeals", map[string]any{
		"reader_id":   readerID,
		"appeal_type": "enrollment_denial",
		"reason":      "Should have been admitted",
	}, cookie)
	require.Equal(t, http.StatusCreated, submitRec.Code)

	var ap map[string]any
	require.NoError(t, json.Unmarshal(submitRec.Body.Bytes(), &ap))
	appealID := ap["id"].(string)

	reviewRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/appeals/"+appealID+"/review", nil, cookie)
	require.Equal(t, http.StatusOK, reviewRec.Code,
		"POST /appeals/:id/review must return 200: body=%s", reviewRec.Body.String())

	var reviewed map[string]any
	require.NoError(t, json.Unmarshal(reviewRec.Body.Bytes(), &reviewed))
	assert.Equal(t, "under_review", reviewed["status"],
		"reviewed appeal must have status=under_review")
}

// TestAppeals_Arbitrate_Upheld verifies the full lifecycle:
// submit → review → arbitrate (upheld) → response has appeal + arbitration with decision.
func TestAppeals_Arbitrate_Upheld(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "APLARB-RDR-"+suffix)

	// Submit.
	submitRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/appeals", map[string]any{
		"reader_id":   readerID,
		"appeal_type": "other",
		"reason":      "Miscellaneous grievance",
	}, cookie)
	require.Equal(t, http.StatusCreated, submitRec.Code)

	var ap map[string]any
	require.NoError(t, json.Unmarshal(submitRec.Body.Bytes(), &ap))
	appealID := ap["id"].(string)

	// Review (transitions to under_review).
	doRequest(t, app.testApp, http.MethodPost, "/api/v1/appeals/"+appealID+"/review", nil, cookie)

	// Arbitrate: upheld.
	arbRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/appeals/"+appealID+"/arbitrate",
		map[string]any{
			"decision":       "upheld",
			"decision_notes": "The reader's complaint is valid",
		},
		cookie,
	)
	require.Equal(t, http.StatusOK, arbRec.Code,
		"POST /appeals/:id/arbitrate must return 200: body=%s", arbRec.Body.String())

	var arbResp map[string]any
	require.NoError(t, json.Unmarshal(arbRec.Body.Bytes(), &arbResp))

	appealField, hasAppeal := arbResp["appeal"]
	assert.True(t, hasAppeal, "arbitrate response must include appeal field")
	if appealMap, ok := appealField.(map[string]any); ok {
		assert.Equal(t, "resolved", appealMap["status"],
			"upheld appeal must have status=resolved")
	}

	arbField, hasArb := arbResp["arbitration"]
	assert.True(t, hasArb, "arbitrate response must include arbitration field")
	if arbMap, ok := arbField.(map[string]any); ok {
		assert.Equal(t, "upheld", arbMap["decision"],
			"arbitration decision must be upheld")
	}
}

// TestAppeals_Arbitrate_Dismissed verifies that a dismissed arbitration decision
// is recorded correctly.
func TestAppeals_Arbitrate_Dismissed(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := contentModCookieForFA(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "APLDIS-RDR-"+suffix)

	submitRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/appeals", map[string]any{
		"reader_id":   readerID,
		"appeal_type": "blacklist_removal",
		"reason":      "I was wrongly blacklisted",
	}, cookie)
	require.Equal(t, http.StatusCreated, submitRec.Code)

	var ap map[string]any
	require.NoError(t, json.Unmarshal(submitRec.Body.Bytes(), &ap))
	appealID := ap["id"].(string)

	doRequest(t, app.testApp, http.MethodPost, "/api/v1/appeals/"+appealID+"/review", nil, cookie)

	arbRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/appeals/"+appealID+"/arbitrate",
		map[string]any{
			"decision":       "dismissed",
			"decision_notes": "Evidence does not support the claim",
		},
		cookie,
	)
	require.Equal(t, http.StatusOK, arbRec.Code,
		"arbitrate with dismissed must return 200: body=%s", arbRec.Body.String())

	var arbResp map[string]any
	require.NoError(t, json.Unmarshal(arbRec.Body.Bytes(), &arbResp))
	arbField, hasArb := arbResp["arbitration"]
	assert.True(t, hasArb, "arbitrate response must include arbitration field")
	if arbMap, ok := arbField.(map[string]any); ok {
		assert.Equal(t, "dismissed", arbMap["decision"],
			"arbitration decision must be dismissed")
	}
}

// TestAppeals_NoArbitratePermission_Returns403 verifies that a user without
// appeals:decide cannot arbitrate.
func TestAppeals_NoArbitratePermission_Returns403(t *testing.T) {
	app := newCompleteTestApp(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-arb-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "")
	roleID := createTempRoleWithPermissions(t, "appeals:read", "appeals:submit") // no appeals:decide
	assignUserRole(t, userID, roleID)
	assignUserToBranch(t, userID, mainBranchID)
	cookie := loginAs(t, app.testApp, username, "Password123!")

	// Submit an appeal first (need a reader and a cookie with submit permission).
	modCookie := contentModCookieForFA(t, app)
	readerID := insertAPITestReader(t, mainBranchID, "NOPERM-APL-RDR-"+suffix)
	submitRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/appeals", map[string]any{
		"reader_id": readerID, "appeal_type": "other", "reason": "Test",
	}, modCookie)
	require.Equal(t, http.StatusCreated, submitRec.Code)

	var ap map[string]any
	require.NoError(t, json.Unmarshal(submitRec.Body.Bytes(), &ap))
	appealID := ap["id"].(string)

	// Attempt to arbitrate with no-permission user.
	rec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/appeals/"+appealID+"/arbitrate",
		map[string]any{"decision": "upheld", "decision_notes": ""},
		cookie,
	)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"user without appeals:decide must get 403 on arbitrate: body=%s", rec.Body.String())
}
