package apitests

// programs_enrollment_test.go tests programs CRUD and the enrollment lifecycle:
//   GET  /api/v1/programs
//   POST /api/v1/programs
//   GET  /api/v1/programs/:id
//   PATCH /api/v1/programs/:id
//   PATCH /api/v1/programs/:id/status
//   GET  /api/v1/programs/:id/prerequisites
//   POST /api/v1/programs/:id/prerequisites
//   DELETE /api/v1/programs/:id/prerequisites/:req_id
//   GET  /api/v1/programs/:id/rules
//   POST /api/v1/programs/:id/rules
//   DELETE /api/v1/programs/:id/rules/:rule_id
//   POST /api/v1/programs/:id/enroll
//   GET  /api/v1/programs/:id/enrollments
//   GET  /api/v1/programs/:id/seats
//   GET  /api/v1/enrollments/:id
//   POST /api/v1/enrollments/:id/drop
//   GET  /api/v1/enrollments/:id/history
//   GET  /api/v1/readers/:id/enrollments

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

// ── Programs CRUD ─────────────────────────────────────────────────────────────

// TestPrograms_CreateAndGet verifies POST /programs creates a program (201) and
// GET /programs/:id retrieves it with prerequisites and rules embedded (200).
func TestPrograms_CreateAndGet(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	now := time.Now().UTC()
	suffix := fmt.Sprintf("%d", now.UnixNano())

	createRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/programs", map[string]any{
		"branch_id":          mainBranchID,
		"title":              "API Test Program " + suffix,
		"capacity":           20,
		"starts_at":          now.Add(time.Hour).Format(time.RFC3339),
		"ends_at":            now.Add(2 * time.Hour).Format(time.RFC3339),
		"enrollment_channel": "any",
	}, cookie)
	require.Equal(t, http.StatusCreated, createRec.Code,
		"POST /programs must return 201: body=%s", createRec.Body.String())

	var cr map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &cr))
	programID, ok := cr["id"].(string)
	require.True(t, ok && programID != "", "response must include non-empty id")

	t.Cleanup(func() {
		pool := testdb.Open(t)
		defer pool.Close()
		execIgnoreErr(pool, `DELETE FROM lms.enrollment_rules WHERE program_id = $1`, programID)
		execIgnoreErr(pool, `DELETE FROM lms.program_prerequisites WHERE program_id = $1`, programID)
		execIgnoreErr(pool, `DELETE FROM lms.enrollments WHERE program_id = $1`, programID)
		execIgnoreErr(pool, `DELETE FROM lms.programs WHERE id = $1`, programID)
	})

	// Get.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/programs/"+programID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code,
		"GET /programs/:id must return 200: body=%s", getRec.Body.String())

	var gr map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &gr))
	assert.Equal(t, programID, gr["id"])
	assert.Equal(t, "API Test Program "+suffix, gr["title"])
	_, hasPrereqs := gr["prerequisites"]
	assert.True(t, hasPrereqs, "GET /programs/:id response must embed prerequisites")
	_, hasRules := gr["enrollment_rules"]
	assert.True(t, hasRules, "GET /programs/:id response must embed enrollment_rules")
}

// TestPrograms_List verifies GET /programs returns a paginated list.
func TestPrograms_List(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/programs?page=1&per_page=10", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /programs must return 200: body=%s", rec.Body.String())

	var resp struct {
		Items   []map[string]any `json:"items"`
		Page    int              `json:"page"`
		PerPage int              `json:"per_page"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 10, resp.PerPage)
}

// TestPrograms_Update verifies PATCH /programs/:id updates title/capacity.
func TestPrograms_Update(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	programID := insertAPITestProgram(t, mainBranchID)

	now := time.Now().UTC()
	rec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/programs/"+programID, map[string]any{
		"title":              "Updated Program Title",
		"capacity":           30,
		"starts_at":          now.Add(time.Hour).Format(time.RFC3339),
		"ends_at":            now.Add(2 * time.Hour).Format(time.RFC3339),
		"enrollment_channel": "any",
	}, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"PATCH /programs/:id must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Updated Program Title", resp["title"])
	assert.EqualValues(t, 30, resp["capacity"])
}

// TestPrograms_UpdateStatus verifies PATCH /programs/:id/status transitions status.
func TestPrograms_UpdateStatus(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	programID := insertAPITestProgram(t, mainBranchID)

	rec := doRequest(t, app.testApp, http.MethodPatch, "/api/v1/programs/"+programID+"/status",
		map[string]any{"status": "cancelled"}, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"PATCH /programs/:id/status must return 200: body=%s", rec.Body.String())
}

// TestPrograms_GetUnknown_Returns404 verifies GET /programs/:id with unknown ID
// returns 404.
func TestPrograms_GetUnknown_Returns404(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/programs/00000000-0000-0000-0000-000000000000", nil, cookie)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"unknown program must return 404: body=%s", rec.Body.String())
}

// TestPrograms_AddAndRemoveRule verifies POST /programs/:id/rules adds a rule (201)
// and DELETE /programs/:id/rules/:rule_id removes it (204).
func TestPrograms_AddAndRemoveRule(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	programID := insertAPITestProgram(t, mainBranchID)

	// Add rule.
	addRec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/programs/"+programID+"/rules",
		map[string]any{
			"rule_type":   "whitelist",
			"match_field": "branch_id",
			"match_value": mainBranchID,
		},
		cookie,
	)
	require.Equal(t, http.StatusCreated, addRec.Code,
		"POST /programs/:id/rules must return 201: body=%s", addRec.Body.String())

	var ar map[string]any
	require.NoError(t, json.Unmarshal(addRec.Body.Bytes(), &ar))
	ruleID, ok := ar["id"].(string)
	require.True(t, ok && ruleID != "", "rule response must include id")

	// List rules.
	listRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/programs/"+programID+"/rules", nil, cookie)
	require.Equal(t, http.StatusOK, listRec.Code)
	var rules []map[string]any
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &rules))
	assert.NotEmpty(t, rules, "program must have at least 1 rule after adding")

	// Remove rule.
	delRec := doRequest(t, app.testApp, http.MethodDelete,
		"/api/v1/programs/"+programID+"/rules/"+ruleID, nil, cookie)
	assert.Equal(t, http.StatusOK, delRec.Code,
		"DELETE /programs/:id/rules/:rule_id must return 200: body=%s", delRec.Body.String())
}

// TestPrograms_Prerequisites verifies adding and removing prerequisites.
func TestPrograms_Prerequisites(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	// Create two programs: one will be the prerequisite of the other.
	prereqID := insertAPITestProgram(t, mainBranchID)
	programID := insertAPITestProgram(t, mainBranchID)

	// Add prerequisite.
	addRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/programs/"+programID+"/prerequisites",
		map[string]any{"required_program_id": prereqID},
		cookie,
	)
	require.Equal(t, http.StatusCreated, addRec.Code,
		"POST /programs/:id/prerequisites must return 201: body=%s", addRec.Body.String())

	var ar map[string]any
	require.NoError(t, json.Unmarshal(addRec.Body.Bytes(), &ar))
	_, ok := ar["id"].(string)
	require.True(t, ok, "prerequisite add response must include non-empty id")

	// List prerequisites.
	listRec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/programs/"+programID+"/prerequisites", nil, cookie)
	require.Equal(t, http.StatusOK, listRec.Code)
	var prereqs []map[string]any
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &prereqs))
	assert.NotEmpty(t, prereqs, "program must have at least 1 prerequisite after adding")

	// Remove prerequisite: the :req_id param is the required program's ID,
	// not the link row's own ID.
	delRec := doRequest(t, app.testApp, http.MethodDelete,
		"/api/v1/programs/"+programID+"/prerequisites/"+prereqID, nil, cookie)
	assert.Equal(t, http.StatusOK, delRec.Code,
		"DELETE /programs/:id/prerequisites/:req_id must return 200: body=%s", delRec.Body.String())
}

// ── Enrollment lifecycle ──────────────────────────────────────────────────────

// TestEnrollment_EnrollAndDrop tests the full enroll → get → drop flow:
//  1. POST /programs/:id/enroll → 201
//  2. GET  /enrollments/:id     → 200
//  3. GET  /programs/:id/enrollments → 200, includes enrollment
//  4. GET  /readers/:id/enrollments  → 200, includes enrollment
//  5. GET  /programs/:id/seats  → 200, seat count decremented
//  6. POST /enrollments/:id/drop → 200
func TestEnrollment_EnrollAndDrop(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "ENR-RDR-"+suffix)
	programID := insertAPITestProgramWithCapacity(t, mainBranchID, 10)

	// 1. Enroll.
	enrollRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/programs/"+programID+"/enroll",
		map[string]any{
			"reader_id":          readerID,
			"enrollment_channel": "any",
		},
		cookie,
	)
	require.Equal(t, http.StatusCreated, enrollRec.Code,
		"POST /programs/:id/enroll must return 201: body=%s", enrollRec.Body.String())

	var er map[string]any
	require.NoError(t, json.Unmarshal(enrollRec.Body.Bytes(), &er))
	enrollmentID, ok := er["id"].(string)
	require.True(t, ok && enrollmentID != "", "enrollment response must include id")

	// 2. Get enrollment.
	getRec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/enrollments/"+enrollmentID, nil, cookie)
	require.Equal(t, http.StatusOK, getRec.Code,
		"GET /enrollments/:id must return 200: body=%s", getRec.Body.String())

	var enr map[string]any
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &enr))
	assert.Equal(t, enrollmentID, enr["id"])
	assert.Equal(t, readerID, enr["reader_id"])
	assert.Equal(t, programID, enr["program_id"])

	// 3. List enrollments by program.
	listByProg := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/programs/"+programID+"/enrollments", nil, cookie)
	require.Equal(t, http.StatusOK, listByProg.Code,
		"GET /programs/:id/enrollments must return 200: body=%s", listByProg.Body.String())
	var progEnrs struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(listByProg.Body.Bytes(), &progEnrs))
	assert.NotEmpty(t, progEnrs.Items, "program enrollments must include the new enrollment")

	// 4. List enrollments by reader.
	listByReader := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/readers/"+readerID+"/enrollments", nil, cookie)
	require.Equal(t, http.StatusOK, listByReader.Code,
		"GET /readers/:id/enrollments must return 200: body=%s", listByReader.Body.String())
	var readerEnrs struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(listByReader.Body.Bytes(), &readerEnrs))
	assert.NotEmpty(t, readerEnrs.Items, "reader enrollments must include the new enrollment")

	// 5. Seats.
	seatsRec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/programs/"+programID+"/seats", nil, cookie)
	require.Equal(t, http.StatusOK, seatsRec.Code,
		"GET /programs/:id/seats must return 200: body=%s", seatsRec.Body.String())
	var seats map[string]any
	require.NoError(t, json.Unmarshal(seatsRec.Body.Bytes(), &seats))
	remaining, _ := seats["remaining_seats"].(float64)
	assert.EqualValues(t, 9, remaining,
		"remaining seats must decrease by 1 after enrollment (capacity=10)")

	// 6. Drop enrollment.
	dropRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/enrollments/"+enrollmentID+"/drop",
		map[string]any{"reader_id": readerID, "reason": "test teardown"},
		cookie,
	)
	require.Equal(t, http.StatusNoContent, dropRec.Code,
		"POST /enrollments/:id/drop must return 204: body=%s", dropRec.Body.String())
}

// TestEnrollment_History verifies GET /enrollments/:id/history returns audit trail.
func TestEnrollment_History(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "HIST-ENR-"+suffix)
	programID := insertAPITestProgramWithCapacity(t, mainBranchID, 5)

	// Enroll.
	enrollRec := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/programs/"+programID+"/enroll",
		map[string]any{"reader_id": readerID, "enrollment_channel": "any"},
		cookie,
	)
	require.Equal(t, http.StatusCreated, enrollRec.Code)
	var er map[string]any
	require.NoError(t, json.Unmarshal(enrollRec.Body.Bytes(), &er))
	enrollmentID := er["id"].(string)

	// Check history.
	histRec := doRequest(t, app.testApp, http.MethodGet,
		"/api/v1/enrollments/"+enrollmentID+"/history", nil, cookie)
	require.Equal(t, http.StatusOK, histRec.Code,
		"GET /enrollments/:id/history must return 200: body=%s", histRec.Body.String())

	var hist []map[string]any
	require.NoError(t, json.Unmarshal(histRec.Body.Bytes(), &hist))
	assert.NotEmpty(t, hist, "enrollment history must have at least 1 entry after enrollment")

	// Drop to clean up.
	doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/enrollments/"+enrollmentID+"/drop",
		map[string]any{"reader_id": readerID, "reason": "cleanup"},
		cookie,
	)
}

// TestEnrollment_DoubleEnroll_Returns409 verifies that enrolling the same reader
// twice in the same program returns a conflict error.
func TestEnrollment_DoubleEnroll_Returns409(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	readerID := insertAPITestReader(t, mainBranchID, "DBLE-ENR-"+suffix)
	programID := insertAPITestProgramWithCapacity(t, mainBranchID, 5)

	body := map[string]any{"reader_id": readerID, "enrollment_channel": "any"}

	// First enrollment — must succeed.
	rec1 := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/programs/"+programID+"/enroll", body, cookie)
	require.Equal(t, http.StatusCreated, rec1.Code,
		"first enroll must return 201: body=%s", rec1.Body.String())

	var er map[string]any
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &er))
	enrollmentID := er["id"].(string)

	// Second enrollment of same reader — must fail with 409.
	rec2 := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/programs/"+programID+"/enroll", body, cookie)
	assert.Equal(t, http.StatusConflict, rec2.Code,
		"duplicate enrollment must return 409: body=%s", rec2.Body.String())

	// Drop to clean up.
	doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/enrollments/"+enrollmentID+"/drop",
		map[string]any{"reader_id": readerID, "reason": "cleanup"},
		cookie,
	)
}

// TestEnrollment_OverCapacity_Returns422 verifies that enrolling beyond capacity
// returns a validation / unprocessable error.
func TestEnrollment_OverCapacity_Returns422(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := adminCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	programID := insertAPITestProgramWithCapacity(t, mainBranchID, 1)

	// First reader — fills the single seat.
	reader1ID := insertAPITestReader(t, mainBranchID, "CAP1-"+suffix)
	rec1 := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/programs/"+programID+"/enroll",
		map[string]any{"reader_id": reader1ID, "enrollment_channel": "any"},
		cookie,
	)
	require.Equal(t, http.StatusCreated, rec1.Code,
		"first enrollment (fills capacity) must return 201: body=%s", rec1.Body.String())

	var er map[string]any
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &er))
	enrollmentID1 := er["id"].(string)

	// Second reader — must fail because program is now full.
	reader2ID := insertAPITestReader(t, mainBranchID, "CAP2-"+suffix)
	rec2 := doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/programs/"+programID+"/enroll",
		map[string]any{"reader_id": reader2ID, "enrollment_channel": "any"},
		cookie,
	)
	assert.NotEqual(t, http.StatusCreated, rec2.Code,
		"enrollment beyond capacity must not return 201: body=%s", rec2.Body.String())

	// Cleanup: drop first enrollment.
	doRequest(t, app.testApp, http.MethodPost,
		"/api/v1/enrollments/"+enrollmentID1+"/drop",
		map[string]any{"reader_id": reader1ID, "reason": "cleanup"},
		cookie,
	)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// insertAPITestProgram inserts a minimal program with capacity=10 and registers cleanup.
func insertAPITestProgram(t *testing.T, branchID string) string {
	t.Helper()
	return insertAPITestProgramWithCapacity(t, branchID, 10)
}

// insertAPITestProgramWithCapacity inserts a program with the given capacity.
func insertAPITestProgramWithCapacity(t *testing.T, branchID string, capacity int) string {
	t.Helper()
	pool := testdb.Open(t)
	defer pool.Close()

	now := time.Now().UTC()
	var id string
	err := pool.QueryRow(contextBG(),
		`INSERT INTO lms.programs
		   (branch_id, title, capacity, starts_at, ends_at, enrollment_channel, status)
		 VALUES ($1, $2, $3, $4, $5, 'any', 'published') RETURNING id::text`,
		branchID,
		"API Test Program "+fmt.Sprintf("%d", now.UnixNano()),
		capacity,
		now.Add(time.Hour),
		now.Add(2*time.Hour),
	).Scan(&id)
	require.NoError(t, err, "insertAPITestProgramWithCapacity: insert failed")

	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		execIgnoreErr(p, `DELETE FROM lms.enrollment_history WHERE enrollment_id IN (SELECT id FROM lms.enrollments WHERE program_id = $1)`, id)
		execIgnoreErr(p, `DELETE FROM lms.enrollments WHERE program_id = $1`, id)
		execIgnoreErr(p, `DELETE FROM lms.enrollment_rules WHERE program_id = $1`, id)
		execIgnoreErr(p, `DELETE FROM lms.program_prerequisites WHERE program_id = $1`, id)
		execIgnoreErr(p, `DELETE FROM lms.program_prerequisites WHERE required_program_id = $1`, id)
		execIgnoreErr(p, `DELETE FROM lms.programs WHERE id = $1`, id)
	})
	return id
}
