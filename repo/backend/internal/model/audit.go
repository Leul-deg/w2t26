package model

import "time"

// AuditEvent is an immutable compliance log entry.
// The application writes audit events via internal/audit/logger.go only.
// Direct writes outside that package are prohibited.
//
// Sensitive values (encrypted field ciphertext, session tokens, passwords)
// must NEVER appear in BeforeValue or AfterValue.
type AuditEvent struct {
	ID            string    `json:"id"`
	EventType     string    `json:"event_type"`
	ActorUserID   *string   `json:"actor_user_id,omitempty"`
	ActorUsername *string   `json:"actor_username,omitempty"` // denormalised for audit durability
	WorkstationID *string   `json:"workstation_id,omitempty"`
	IPAddress     *string   `json:"ip_address,omitempty"`
	BranchID      *string   `json:"branch_id,omitempty"`
	ResourceType  *string   `json:"resource_type,omitempty"`
	ResourceID    *string   `json:"resource_id,omitempty"`
	BeforeValue   any       `json:"before_value,omitempty"` // JSONB snapshot
	AfterValue    any       `json:"after_value,omitempty"`  // JSONB snapshot
	Metadata      any       `json:"metadata,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// AuditEventType constants for use across all packages.
const (
	AuditLoginSuccess        = "auth.login.success"
	AuditLoginFailure        = "auth.login.failure"
	AuditLoginLocked         = "auth.login.locked"
	AuditLogout              = "auth.logout"
	AuditSessionExpired      = "auth.session.expired"
	AuditUserCreated         = "user.created"
	AuditUserUpdated         = "user.updated"
	AuditUserRoleChanged     = "user.role.changed"
	AuditUserBranchChanged   = "user.branch.changed"
	AuditReaderCreated       = "reader.created"
	AuditReaderUpdated       = "reader.updated"
	AuditReaderStatusChanged = "reader.status.changed"
	AuditSensitiveRevealed   = "reader.sensitive.revealed"
	AuditExportCreated       = "export.created"
	AuditImportCommitted     = "import.committed"
	AuditImportRolledBack    = "import.rolled_back"
	AuditModerationDecision  = "moderation.decision"
	AuditAppealDecision      = "appeal.decision"
	AuditContentSubmitted    = "content.submitted"
	AuditFeedbackSubmitted   = "feedback.submitted"
	AuditFeedbackModerated   = "feedback.moderated"
	AuditArbitrationDecided  = "arbitration.decided"
	AuditEnrollmentChanged   = "enrollment.status.changed"

	// Holdings / copies
	AuditHoldingCreated     = "holding.created"
	AuditHoldingUpdated     = "holding.updated"
	AuditHoldingDeactivated = "holding.deactivated"
	AuditCopyCreated        = "copy.created"
	AuditCopyUpdated        = "copy.updated"
	AuditCopyStatusChanged  = "copy.status_changed"

	// Stocktake
	AuditStocktakeCreated = "stocktake.session.created"
	AuditStocktakeClosed  = "stocktake.session.closed"
	AuditStocktakeScan    = "stocktake.finding.recorded"

	// Programs
	AuditProgramCreated       = "program.created"
	AuditProgramUpdated       = "program.updated"
	AuditProgramStatusChanged = "program.status.changed"

	// Circulation
	AuditCirculationCheckout = "circulation.checkout"
	AuditCirculationReturn   = "circulation.return"
)
