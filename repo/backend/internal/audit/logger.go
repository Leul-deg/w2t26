// Package audit provides the centralised audit event logger.
// All compliance writes must go through this package.
// Direct writes to the audit_events table from other packages are prohibited.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"lms/internal/domain/audit"
	"lms/internal/model"
)

// Logger writes structured audit events via an audit.Repository.
// It never panics; if the repository returns an error it is logged with slog
// and execution continues. Sensitive values (passwords, tokens, ciphertext)
// must never appear in any audit event.
type Logger struct {
	repo audit.Repository
}

// New creates a new Logger backed by the provided repository.
func New(repo audit.Repository) *Logger {
	return &Logger{repo: repo}
}

// Log writes a single audit event. If the repository returns an error the
// error is logged via slog and the function returns normally — audit failures
// must never block business operations.
func (l *Logger) Log(ctx context.Context, e *model.AuditEvent) {
	if e.ID == "" {
		e.ID = newUUID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if err := l.repo.Insert(ctx, e); err != nil {
		slog.Error("audit: failed to insert event",
			"event_type", e.EventType,
			"actor_user_id", e.ActorUserID,
			"error", err,
		)
	}
}

// LogLogin records a successful login event.
func (l *Logger) LogLogin(ctx context.Context, userID, username, workstationID, ipAddr string) {
	e := &model.AuditEvent{
		EventType:     model.AuditLoginSuccess,
		ActorUserID:   strPtr(userID),
		ActorUsername: strPtr(username),
		WorkstationID: strPtr(workstationID),
		IPAddress:     strPtr(ipAddr),
	}
	l.Log(ctx, e)
}

// LogLoginFailed records a failed login attempt.
func (l *Logger) LogLoginFailed(ctx context.Context, username, workstationID, ipAddr, reason string) {
	// username is logged for audit purposes (not a secret) but no password or
	// token data is included.
	e := &model.AuditEvent{
		EventType:     model.AuditLoginFailure,
		ActorUsername: strPtr(username),
		WorkstationID: strPtr(workstationID),
		IPAddress:     strPtr(ipAddr),
		Metadata:      map[string]string{"reason": reason},
	}
	l.Log(ctx, e)
}

// LogLockout records an account lockout event.
func (l *Logger) LogLockout(ctx context.Context, userID, username string) {
	e := &model.AuditEvent{
		EventType:     model.AuditLoginLocked,
		ActorUserID:   strPtr(userID),
		ActorUsername: strPtr(username),
	}
	l.Log(ctx, e)
}

// LogLogout records a logout event for the given session.
func (l *Logger) LogLogout(ctx context.Context, userID, username, sessionID string) {
	e := &model.AuditEvent{
		EventType:     model.AuditLogout,
		ActorUserID:   strPtr(userID),
		ActorUsername: strPtr(username),
		// Session ID is not sensitive (it's an internal UUID, not the token).
		Metadata: map[string]string{"session_id": sessionID},
	}
	l.Log(ctx, e)
}

// LogAdminChange records an administrative change to a resource.
// eventType must be one of the AuditXxx constants in model/audit.go.
// workstationID is the value of the X-Workstation-ID request header, or "" if unavailable.
// before and after must NOT contain sensitive field values (passwords,
// encrypted ciphertext, session tokens).
func (l *Logger) LogAdminChange(ctx context.Context, actorID, actorUsername, eventType, resourceType, resourceID, workstationID string, before, after any) {
	e := &model.AuditEvent{
		EventType:     eventType,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		WorkstationID: strPtr(workstationID),
		ResourceType:  strPtr(resourceType),
		ResourceID:    strPtr(resourceID),
		BeforeValue:   sanitize(before),
		AfterValue:    sanitize(after),
	}
	l.Log(ctx, e)
}

// LogCopyChange records a create, update, or status change on a physical copy.
// before/after snapshots must not contain barcodes of other readers or sensitive data.
func (l *Logger) LogCopyChange(ctx context.Context, actorID, actorUsername, eventType, copyID, branchID string, before, after any) {
	e := &model.AuditEvent{
		EventType:     eventType,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		BranchID:      strPtr(branchID),
		ResourceType:  strPtr("copy"),
		ResourceID:    strPtr(copyID),
		BeforeValue:   sanitize(before),
		AfterValue:    sanitize(after),
	}
	l.Log(ctx, e)
}

// LogStocktakeScan records a barcode scan within a stocktake session.
func (l *Logger) LogStocktakeScan(ctx context.Context, actorID, actorUsername, sessionID, barcode, findingType string) {
	e := &model.AuditEvent{
		EventType:     model.AuditStocktakeScan,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		ResourceType:  strPtr("stocktake_session"),
		ResourceID:    strPtr(sessionID),
		Metadata:      map[string]string{"barcode": barcode, "finding_type": findingType},
	}
	l.Log(ctx, e)
}

// LogStocktakeEvent records a stocktake session lifecycle event (created/closed/cancelled).
func (l *Logger) LogStocktakeEvent(ctx context.Context, actorID, actorUsername, eventType, sessionID, branchID string) {
	e := &model.AuditEvent{
		EventType:     eventType,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		BranchID:      strPtr(branchID),
		ResourceType:  strPtr("stocktake_session"),
		ResourceID:    strPtr(sessionID),
	}
	l.Log(ctx, e)
}

// LogEnrollmentChanged records a reader enrollment status change (enroll, drop, etc.).
func (l *Logger) LogEnrollmentChanged(ctx context.Context, actorID, actorUsername, enrollmentID, programID, prevStatus, newStatus, reason string) {
	e := &model.AuditEvent{
		EventType:     model.AuditEnrollmentChanged,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		ResourceType:  strPtr("enrollment"),
		ResourceID:    strPtr(enrollmentID),
		BeforeValue:   map[string]string{"status": prevStatus},
		AfterValue:    map[string]string{"status": newStatus},
		Metadata:      map[string]string{"program_id": programID, "reason": reason},
	}
	l.Log(ctx, e)
}

// LogImportEvent records an import lifecycle event (committed or rolled_back).
func (l *Logger) LogImportEvent(ctx context.Context, actorID, actorUsername, eventType, jobID, branchID string, metadata any) {
	e := &model.AuditEvent{
		EventType:     eventType,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		BranchID:      strPtr(branchID),
		ResourceType:  strPtr("import_job"),
		ResourceID:    strPtr(jobID),
		Metadata:      sanitize(metadata),
	}
	l.Log(ctx, e)
}

// LogExportCreated records a successful export operation.
// rowCount is the number of rows included in the export file.
func (l *Logger) LogExportCreated(ctx context.Context, actorID, actorUsername, jobID, exportType, branchID string, rowCount int) {
	e := &model.AuditEvent{
		EventType:     model.AuditExportCreated,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		BranchID:      strPtr(branchID),
		ResourceType:  strPtr("export_job"),
		ResourceID:    strPtr(jobID),
		Metadata:      map[string]any{"export_type": exportType, "row_count": rowCount},
	}
	l.Log(ctx, e)
}

// LogModerationDecision records a moderator's decision on a governed content item.
func (l *Logger) LogModerationDecision(ctx context.Context, actorID, actorUsername, contentID, decision, reason string) {
	e := &model.AuditEvent{
		EventType:     model.AuditModerationDecision,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		ResourceType:  strPtr("governed_content"),
		ResourceID:    strPtr(contentID),
		AfterValue:    map[string]string{"decision": decision},
		Metadata:      map[string]string{"reason": reason},
	}
	l.Log(ctx, e)
}

// LogAppealDecided records the outcome of an appeal arbitration.
func (l *Logger) LogAppealDecided(ctx context.Context, actorID, actorUsername, appealID, decision, notes string) {
	e := &model.AuditEvent{
		EventType:     model.AuditAppealDecision,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		ResourceType:  strPtr("appeal"),
		ResourceID:    strPtr(appealID),
		AfterValue:    map[string]string{"decision": decision},
		Metadata:      map[string]string{"notes": notes},
	}
	l.Log(ctx, e)
}

// LogFeedbackSubmitted records a new feedback submission.
func (l *Logger) LogFeedbackSubmitted(ctx context.Context, actorID, actorUsername, feedbackID, readerID, targetType, targetID string) {
	e := &model.AuditEvent{
		EventType:     model.AuditFeedbackSubmitted,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		ResourceType:  strPtr("feedback"),
		ResourceID:    strPtr(feedbackID),
		Metadata: map[string]string{
			"reader_id":   readerID,
			"target_type": targetType,
			"target_id":   targetID,
		},
	}
	l.Log(ctx, e)
}

// LogFeedbackModerated records a moderation decision on a feedback item.
func (l *Logger) LogFeedbackModerated(ctx context.Context, actorID, actorUsername, feedbackID, status string) {
	e := &model.AuditEvent{
		EventType:     model.AuditFeedbackModerated,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		ResourceType:  strPtr("feedback"),
		ResourceID:    strPtr(feedbackID),
		AfterValue:    map[string]string{"status": status},
	}
	l.Log(ctx, e)
}

// LogContentSubmitted records that a draft content item entered the moderation workflow.
func (l *Logger) LogContentSubmitted(ctx context.Context, actorID, actorUsername, contentID string) {
	e := &model.AuditEvent{
		EventType:     model.AuditContentSubmitted,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		ResourceType:  strPtr("governed_content"),
		ResourceID:    strPtr(contentID),
		AfterValue:    map[string]string{"status": "pending_review"},
	}
	l.Log(ctx, e)
}

// LogSensitiveReveal records that a staff member revealed a reader's encrypted fields.
func (l *Logger) LogSensitiveReveal(ctx context.Context, actorID, actorUsername, readerID string) {
	e := &model.AuditEvent{
		EventType:     model.AuditSensitiveRevealed,
		ActorUserID:   strPtr(actorID),
		ActorUsername: strPtr(actorUsername),
		ResourceType:  strPtr("reader"),
		ResourceID:    strPtr(readerID),
		// No before/after values — decrypted data must never appear in audit events.
	}
	l.Log(ctx, e)
}

// strPtr returns a pointer to the string, or nil if the string is empty.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// sanitize converts a value to a JSON-safe representation. It round-trips
// through encoding/json so any unexportable fields are stripped. This is a
// safety measure — callers should already ensure no sensitive data is passed.
func sanitize(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

// newUUID generates a random UUID v4 string using math/rand as a fallback.
// The audit logger uses the database's uuid_generate_v4() via the INSERT RETURNING id,
// so this is only used as a placeholder before the DB assigns the real ID.
// In practice, e.ID is set from the DB RETURNING clause after Insert succeeds.
func newUUID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(rand.Intn(256)) //nolint:gosec // non-security use
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
