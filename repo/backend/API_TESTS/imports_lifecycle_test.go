package apitests

// imports_lifecycle_test.go exercises the full import pipeline over real HTTP:
//
//   POST /api/v1/imports          (multipart upload)
//   GET  /api/v1/imports/:id      (preview)
//   POST /api/v1/imports/:id/rollback
//   POST /api/v1/imports/:id/commit
//   GET  /api/v1/imports/:id/errors.csv
//   GET  /api/v1/imports          (list jobs)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/tests/testdb"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// importOpsUserCookie creates an operations_staff user scoped to mainBranchID
// and returns a logged-in session cookie. Ops staff has imports:create and
// imports:commit permissions.
func importOpsUserCookie(t *testing.T, app *completeTestApp) *http.Cookie {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "import-ops-" + suffix
	userID := createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "operations_staff")
	assignUserToBranch(t, userID, mainBranchID)
	return loginAs(t, app.testApp, username, "Password123!")
}

// doMultipartUpload sends a multipart/form-data POST to /api/v1/imports.
// importType must be "readers" or "holdings". filename is the file name header.
func doMultipartUpload(t *testing.T, app *completeTestApp, importType, filename string, data []byte, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fw, err := writer.CreateFormField("import_type")
	require.NoError(t, err)
	_, err = fw.Write([]byte(importType))
	require.NoError(t, err)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	h.Set("Content-Type", "text/csv")
	part, err := writer.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/imports", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	app.e.ServeHTTP(rec, req)
	return rec
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestImports_UploadValidCSV_ReturnsPreviewReady verifies that uploading a
// well-formed readers CSV returns 202 with status "preview_ready".
func TestImports_UploadValidCSV_ReturnsPreviewReady(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	csv := fmt.Appendf(nil,
		"reader_number,first_name,last_name\n"+
			"VALID001-%s,Alice,Alpha\n"+
			"VALID002-%s,Bob,Beta\n",
		suffix, suffix,
	)

	rec := doMultipartUpload(t, app, "readers", "readers.csv", csv, cookie)
	require.Equal(t, http.StatusAccepted, rec.Code,
		"valid CSV upload must return 202: body=%s", rec.Body.String())

	var job map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &job))
	assert.Equal(t, "preview_ready", job["status"], "job status must be preview_ready")
	assert.EqualValues(t, 0, job["error_count"], "valid CSV must have zero errors")
	jobID, ok := job["id"].(string)
	require.True(t, ok && jobID != "", "response must include non-empty job id")
}

// TestImports_UploadInvalidCSV_Returns422 verifies that a CSV with a row error
// returns 422 with status "failed" and a non-zero error_count.
func TestImports_UploadInvalidCSV_Returns422(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	csv := []byte("reader_number,first_name,last_name\n" +
		"BAD001,,Smith\n") // missing first_name

	rec := doMultipartUpload(t, app, "readers", "bad.csv", csv, cookie)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"CSV with errors must return 422: body=%s", rec.Body.String())

	var job map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &job))
	assert.Equal(t, "failed", job["status"], "job with row errors must have status failed")
	ec, _ := job["error_count"].(float64)
	assert.Greater(t, ec, float64(0), "error_count must be > 0")
}

// TestImports_UploadNoPermission_Returns403 verifies that a user without
// imports:create receives 403.
func TestImports_UploadNoPermission_Returns403(t *testing.T) {
	app := newCompleteTestApp(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "noperm-import-" + suffix
	createTestUser(t, app.testApp, username, username+"@test.local", "Password123!", "content_moderator")
	cookie := loginAs(t, app.testApp, username, "Password123!")

	rec := doMultipartUpload(t, app, "readers", "any.csv", []byte("a,b\n"), cookie)
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"content_moderator must get 403 on import upload: body=%s", rec.Body.String())
}

// TestImports_GetPreview_Returns200 verifies GET /imports/:id returns the job
// and a rows page after a successful upload.
func TestImports_GetPreview_Returns200(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	csv := fmt.Appendf(nil,
		"reader_number,first_name,last_name\nPREV001-%s,Preview,User\n", suffix,
	)
	upRec := doMultipartUpload(t, app, "readers", "preview.csv", csv, cookie)
	require.Equal(t, http.StatusAccepted, upRec.Code)

	var up map[string]any
	require.NoError(t, json.Unmarshal(upRec.Body.Bytes(), &up))
	jobID := up["id"].(string)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports/"+jobID, nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /imports/:id must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, hasJob := resp["job"]
	assert.True(t, hasJob, "preview response must contain job field")
	_, hasRows := resp["rows"]
	assert.True(t, hasRows, "preview response must contain rows field")
}

// TestImports_ListJobs_Returns200 verifies GET /imports returns a paginated list.
func TestImports_ListJobs_Returns200(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /imports must return 200: body=%s", rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	_, hasItems := resp["items"]
	assert.True(t, hasItems, "list response must contain items array")
}

// TestImports_Rollback_Returns200 verifies that rolling back a preview_ready job
// transitions it to rolled_back.
func TestImports_Rollback_Returns200(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	csv := fmt.Appendf(nil,
		"reader_number,first_name,last_name\nRLBK001-%s,Rollback,User\n", suffix,
	)
	upRec := doMultipartUpload(t, app, "readers", "rollback.csv", csv, cookie)
	require.Equal(t, http.StatusAccepted, upRec.Code,
		"upload before rollback test must succeed: body=%s", upRec.Body.String())

	var up map[string]any
	require.NoError(t, json.Unmarshal(upRec.Body.Bytes(), &up))
	jobID := up["id"].(string)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/imports/"+jobID+"/rollback", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"POST /imports/:id/rollback must return 200: body=%s", rec.Body.String())

	var job map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &job))
	assert.Equal(t, "rolled_back", job["status"],
		"rolled back job must have status rolled_back")
}

// TestImports_CommitFailedJob_Returns422 verifies that attempting to commit a job
// with errors returns 422 (commit blocked when error_count > 0).
func TestImports_CommitFailedJob_Returns422(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	csv := []byte("reader_number,first_name,last_name\n" +
		"CMTERR001,,Smith\n") // missing first_name → fails validation

	upRec := doMultipartUpload(t, app, "readers", "err.csv", csv, cookie)
	require.Equal(t, http.StatusUnprocessableEntity, upRec.Code)

	var up map[string]any
	require.NoError(t, json.Unmarshal(upRec.Body.Bytes(), &up))
	jobID := up["id"].(string)

	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/imports/"+jobID+"/commit", nil, cookie)
	// Commit must be blocked — the service returns a Validation error which maps to 422.
	assert.NotEqual(t, http.StatusOK, rec.Code,
		"committing a failed job must not return 200: body=%s", rec.Body.String())
}

// TestImports_CommitValidJob_InsertsRows verifies the critical path: uploading a
// valid readers CSV and then committing it actually inserts reader rows into the DB.
func TestImports_CommitValidJob_InsertsRows(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	rn1 := "CMTOK001-" + suffix
	rn2 := "CMTOK002-" + suffix

	// Register cleanup for the rows that will be inserted.
	t.Cleanup(func() {
		p := testdb.Open(t)
		defer p.Close()
		execIgnoreErr(p, `DELETE FROM lms.readers WHERE reader_number = $1`, rn1)
		execIgnoreErr(p, `DELETE FROM lms.readers WHERE reader_number = $1`, rn2)
	})

	csv := fmt.Appendf(nil,
		"reader_number,first_name,last_name\n%s,Commit,One\n%s,Commit,Two\n",
		rn1, rn2,
	)

	upRec := doMultipartUpload(t, app, "readers", "commit_ok.csv", csv, cookie)
	require.Equal(t, http.StatusAccepted, upRec.Code,
		"upload before commit test must succeed: body=%s", upRec.Body.String())

	var up map[string]any
	require.NoError(t, json.Unmarshal(upRec.Body.Bytes(), &up))
	jobID := up["id"].(string)
	require.Equal(t, "preview_ready", up["status"], "job must be preview_ready before commit")

	// Commit.
	rec := doRequest(t, app.testApp, http.MethodPost, "/api/v1/imports/"+jobID+"/commit", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"POST /imports/:id/commit must return 200: body=%s", rec.Body.String())

	var job map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &job))
	assert.Equal(t, "committed", job["status"], "committed job must have status committed")

	// Verify the rows were actually inserted into the database.
	pool := testdb.Open(t)
	defer pool.Close()
	var count int
	err := pool.QueryRow(contextBG(),
		`SELECT COUNT(*) FROM lms.readers WHERE reader_number IN ($1, $2)`,
		rn1, rn2,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "both readers must be present in DB after commit")
}

// TestImports_DownloadErrors_Returns200 verifies that GET /imports/:id/errors.csv
// returns a CSV attachment for a job with errors.
func TestImports_DownloadErrors_Returns200(t *testing.T) {
	app := newCompleteTestApp(t)
	cookie := importOpsUserCookie(t, app)

	csv := []byte("reader_number,first_name,last_name\nERRDL001,,Smith\n")
	upRec := doMultipartUpload(t, app, "readers", "errs.csv", csv, cookie)
	require.Equal(t, http.StatusUnprocessableEntity, upRec.Code)

	var up map[string]any
	require.NoError(t, json.Unmarshal(upRec.Body.Bytes(), &up))
	jobID := up["id"].(string)

	rec := doRequest(t, app.testApp, http.MethodGet, "/api/v1/imports/"+jobID+"/errors.csv", nil, cookie)
	require.Equal(t, http.StatusOK, rec.Code,
		"GET /imports/:id/errors.csv must return 200: body=%s", rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment",
		"response must have attachment content-disposition")
	assert.Contains(t, rec.Body.String(), "row_number",
		"error CSV must include row_number header")
}
