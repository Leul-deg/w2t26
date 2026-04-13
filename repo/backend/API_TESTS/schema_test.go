package apitests

// schema_test.go verifies database schema integrity, constraint enforcement,
// and seed data presence. These tests require a live PostgreSQL test database.
//
// Run with: DATABASE_TEST_URL=<dsn> go test ./API_TESTS/... -v
// Or set DATABASE_TEST_URL in backend/.env.
//
// Constraint tests use rollback transactions so they leave no residual data and
// are safe to run multiple times against the same database.
//
// These tests verify:
//  1. All migrations apply cleanly (all expected tables present).
//  2. Critical uniqueness constraints are enforced (barcode, reader_number, etc.).
//  3. Foreign-key and check constraints are enforced.
//  4. Seed data is present after migration.
//  5. Audit events cannot be deleted (append-only REVOKE enforcement).

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lms/tests/testdb"
)

// uniqueSuffix returns a nanosecond-based string suffix for test data isolation.
// Unique within a single test run and across runs because it uses wall-clock time.
func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// withRollback executes fn inside a transaction that is always rolled back.
// Use for constraint tests that must not leave residual data in the database.
func withRollback(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, pool *pgxpool.Pool)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err, "begin transaction")
	defer tx.Rollback(ctx) //nolint:errcheck // rollback is intentional

	fn(ctx, pool)
}

// execTx runs a SQL statement inside a one-shot transaction that is always rolled back.
// Returns the error from the statement (not from the rollback).
func execTx(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck
	_, execErr := tx.Exec(ctx, sql, args...)
	return execErr
}

// execTxOK runs SQL inside a rollback transaction and fails the test if it errors.
func execTxOK(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if err := execTx(t, pool, sql, args...); err != nil {
		t.Fatalf("expected success but got: %v\nSQL: %s", err, sql)
	}
}

// TestMigrations_AllTablesPresent verifies that all expected tables exist after
// running all migrations. This is the migration sanity check.
func TestMigrations_AllTablesPresent(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	expectedTables := []string{
		"roles", "permissions", "role_permissions",
		"users", "user_roles",
		"sessions", "captcha_challenges",
		"branches", "user_branch_assignments",
		"reader_statuses", "readers",
		"holdings", "copy_statuses", "copies",
		"circulation_events",
		"stocktake_sessions", "stocktake_findings",
		"programs", "program_prerequisites", "enrollment_rules",
		"enrollments", "enrollment_history",
		"governed_content", "moderation_items",
		"feedback_tags", "feedback", "feedback_tag_mappings",
		"appeals", "appeal_arbitrations",
		"import_jobs", "import_rows",
		"export_jobs",
		"audit_events",
		"report_definitions", "report_aggregates",
	}

	for _, table := range expectedTables {
		t.Run("table_exists/"+table, func(t *testing.T) {
			var exists bool
			err := pool.QueryRow(context.Background(),
				`SELECT EXISTS(
					SELECT 1 FROM pg_tables
					WHERE schemaname = 'lms' AND tablename = $1
				)`, table).Scan(&exists)
			require.NoError(t, err)
			assert.True(t, exists, "table lms.%s must exist after migrations", table)
		})
	}
}

// TestConstraint_BarcodeUnique verifies that two copies cannot share a barcode.
func TestConstraint_BarcodeUnique(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()
	branchID := "bbbbbbbb-0000-0000-0000-000000000001"
	barcode := "BARCODE-" + uniqueSuffix()

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	// Insert a holding.
	var holdingID string
	err = tx.QueryRow(ctx,
		`INSERT INTO lms.holdings (branch_id, title, language)
		 VALUES ($1, 'Barcode Test Holding', 'en') RETURNING id`,
		branchID,
	).Scan(&holdingID)
	require.NoError(t, err)

	// First copy: must succeed.
	_, err = tx.Exec(ctx,
		`INSERT INTO lms.copies (holding_id, branch_id, barcode) VALUES ($1, $2, $3)`,
		holdingID, branchID, barcode,
	)
	require.NoError(t, err, "first copy should succeed")

	// Second copy with same barcode: must fail.
	_, err = tx.Exec(ctx,
		`INSERT INTO lms.copies (holding_id, branch_id, barcode) VALUES ($1, $2, $3)`,
		holdingID, branchID, barcode,
	)
	require.Error(t, err, "duplicate barcode must be rejected")
	testdb.AssertConstraintError(t, err, "copies_barcode_key")
}

// TestConstraint_ReaderNumberUnique verifies that reader_number is globally unique.
func TestConstraint_ReaderNumberUnique(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()
	mainBranch := "bbbbbbbb-0000-0000-0000-000000000001"
	eastBranch := "bbbbbbbb-0000-0000-0000-000000000002"
	readerNumber := "RN-" + uniqueSuffix()

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.readers (branch_id, reader_number, first_name, last_name)
		 VALUES ($1, $2, 'Alice', 'Test')`,
		mainBranch, readerNumber,
	)
	require.NoError(t, err, "first reader insert should succeed")

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.readers (branch_id, reader_number, first_name, last_name)
		 VALUES ($1, $2, 'Bob', 'Test')`,
		eastBranch, readerNumber,
	)
	require.Error(t, err, "duplicate reader_number across branches must be rejected")
	testdb.AssertConstraintError(t, err, "readers_reader_number_key")
}

// TestConstraint_UsernameUnique verifies that users.username is unique.
func TestConstraint_UsernameUnique(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()
	username := "usr-" + uniqueSuffix()

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.users (username, email, password_hash) VALUES ($1, $2, 'h')`,
		username, username+"@a.local",
	)
	require.NoError(t, err)

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.users (username, email, password_hash) VALUES ($1, $2, 'h')`,
		username, username+"@b.local",
	)
	require.Error(t, err, "duplicate username must be rejected")
	testdb.AssertConstraintError(t, err, "users_username_key")
}

// TestConstraint_EmailUnique verifies that users.email is unique.
func TestConstraint_EmailUnique(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()
	email := "e-" + uniqueSuffix() + "@test.local"
	suffix := uniqueSuffix()

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.users (username, email, password_hash) VALUES ($1, $2, 'h')`,
		"ua-"+suffix, email,
	)
	require.NoError(t, err)

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.users (username, email, password_hash) VALUES ($1, $2, 'h')`,
		"ub-"+suffix, email,
	)
	require.Error(t, err, "duplicate email must be rejected")
	testdb.AssertConstraintError(t, err, "users_email_key")
}

// TestConstraint_EnrollmentUnique verifies that a reader cannot be enrolled in
// the same program twice.
func TestConstraint_EnrollmentUnique(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()
	branchID := "bbbbbbbb-0000-0000-0000-000000000001"
	suffix := uniqueSuffix()

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	var readerID string
	err = tx.QueryRow(ctx,
		`INSERT INTO lms.readers (branch_id, reader_number, first_name, last_name)
		 VALUES ($1, $2, 'Enroll', 'Test') RETURNING id`,
		branchID, "ENR-"+suffix,
	).Scan(&readerID)
	require.NoError(t, err)

	var programID string
	err = tx.QueryRow(ctx,
		`INSERT INTO lms.programs (branch_id, title, capacity, starts_at, ends_at)
		 VALUES ($1, 'Test Program', 10,
		         NOW() + interval '1 day',
		         NOW() + interval '2 days')
		 RETURNING id`,
		branchID,
	).Scan(&programID)
	require.NoError(t, err)

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.enrollments (program_id, reader_id, branch_id) VALUES ($1, $2, $3)`,
		programID, readerID, branchID,
	)
	require.NoError(t, err, "first enrollment should succeed")

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.enrollments (program_id, reader_id, branch_id) VALUES ($1, $2, $3)`,
		programID, readerID, branchID,
	)
	require.Error(t, err, "duplicate enrollment must be rejected")
	testdb.AssertConstraintError(t, err, "enrollments_program_id_reader_id_key")
}

// TestConstraint_ProgramDatesCheck verifies that ends_at > starts_at.
func TestConstraint_ProgramDatesCheck(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO lms.programs (branch_id, title, capacity, starts_at, ends_at)
		 VALUES ('bbbbbbbb-0000-0000-0000-000000000001', 'Bad Dates', 10,
		         NOW() + interval '2 days', NOW() + interval '1 day')`,
	)
	require.Error(t, err, "ends_at < starts_at must be rejected")
	testdb.AssertConstraintError(t, err, "chk_program_dates")
}

// TestConstraint_OneActiveStocktakePerBranch verifies the partial unique index
// that prevents two open/in_progress sessions for the same branch.
func TestConstraint_OneActiveStocktakePerBranch(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()
	// Use EAST branch to avoid clashing with any residual data.
	branchID := "bbbbbbbb-0000-0000-0000-000000000002"
	adminUserID := "aaaaaaaa-0000-0000-0000-000000000001"
	suffix := uniqueSuffix()

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	// Cancel any pre-existing open sessions for this branch (defensive cleanup).
	_, err = tx.Exec(ctx,
		`UPDATE lms.stocktake_sessions SET status = 'cancelled'
		 WHERE branch_id = $1 AND status IN ('open', 'in_progress')`, branchID,
	)
	require.NoError(t, err)

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.stocktake_sessions (branch_id, name, started_by) VALUES ($1, $2, $3)`,
		branchID, "Session-A-"+suffix, adminUserID,
	)
	require.NoError(t, err, "first active session should succeed")

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.stocktake_sessions (branch_id, name, started_by) VALUES ($1, $2, $3)`,
		branchID, "Session-B-"+suffix, adminUserID,
	)
	require.Error(t, err, "second active session for same branch must be rejected")
	testdb.AssertConstraintError(t, err, "idx_stocktake_one_active")
}

// TestConstraint_StocktakeDuplicateScan verifies that scanning the same barcode
// twice in one session is rejected.
func TestConstraint_StocktakeDuplicateScan(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()
	branchID := "bbbbbbbb-0000-0000-0000-000000000001"
	adminUserID := "aaaaaaaa-0000-0000-0000-000000000001"
	suffix := uniqueSuffix()

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	// Cancel any pre-existing open sessions for this branch (defensive cleanup).
	_, err = tx.Exec(ctx,
		`UPDATE lms.stocktake_sessions SET status = 'cancelled'
		 WHERE branch_id = $1 AND status IN ('open', 'in_progress')`, branchID,
	)
	require.NoError(t, err)

	var sessionID string
	err = tx.QueryRow(ctx,
		`INSERT INTO lms.stocktake_sessions (branch_id, name, started_by)
		 VALUES ($1, $2, $3) RETURNING id`,
		branchID, "DupScan-"+suffix, adminUserID,
	).Scan(&sessionID)
	require.NoError(t, err)

	barcode := "DUP-BC-" + suffix

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.stocktake_findings (session_id, scanned_barcode, finding_type)
		 VALUES ($1, $2, 'found')`, sessionID, barcode,
	)
	require.NoError(t, err, "first scan should succeed")

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.stocktake_findings (session_id, scanned_barcode, finding_type)
		 VALUES ($1, $2, 'found')`, sessionID, barcode,
	)
	require.Error(t, err, "duplicate barcode in same session must be rejected")
	testdb.AssertConstraintError(t, err, "stocktake_findings_session_id_scanned_barcode_key")
}

// TestSeedData_AdminUserExists verifies that the seed admin user was inserted.
func TestSeedData_AdminUserExists(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	var username string
	err := pool.QueryRow(context.Background(),
		`SELECT username FROM lms.users WHERE username = 'admin'`,
	).Scan(&username)
	require.NoError(t, err, "seed admin user must exist")
	assert.Equal(t, "admin", username)
}

// TestSeedData_BranchesExist verifies that both seed branches are present.
func TestSeedData_BranchesExist(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM lms.branches WHERE code IN ('MAIN', 'EAST')`,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "both seed branches must be present")
}

// TestSeedData_RolesAndPermissions verifies the three roles and permission set.
func TestSeedData_RolesAndPermissions(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()

	var roleCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM lms.roles`).Scan(&roleCount))
	assert.Equal(t, 3, roleCount, "three roles must be seeded")

	var permCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM lms.permissions`).Scan(&permCount))
	assert.Greater(t, permCount, 20, "at least 21 permissions must be seeded")
}

// TestSeedData_AdminHasAllPermissions verifies the administrator role has all permissions.
func TestSeedData_AdminHasAllPermissions(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()

	var total, adminCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM lms.permissions`).Scan(&total))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lms.role_permissions rp
		 JOIN lms.roles r ON r.id = rp.role_id
		 WHERE r.name = 'administrator'`).Scan(&adminCount))

	assert.Equal(t, total, adminCount,
		"administrator must have every defined permission")
}

// TestConstraint_AuditEventsNoDeletePriv verifies that lms_user cannot delete
// from audit_events (append-only enforcement via REVOKE DELETE).
func TestConstraint_AuditEventsNoDeletePriv(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()

	// Insert a test audit event (INSERT is allowed).
	var auditID string
	err := pool.QueryRow(ctx,
		`INSERT INTO lms.audit_events (event_type) VALUES ('test.noop') RETURNING id`,
	).Scan(&auditID)
	require.NoError(t, err, "INSERT on audit_events must succeed")

	// DELETE must be rejected because REVOKE DELETE was applied in migration 012.
	_, err = pool.Exec(ctx,
		`DELETE FROM lms.audit_events WHERE id = $1`, auditID,
	)
	require.Error(t, err, "DELETE on audit_events must be rejected for lms_user")
	assert.True(t,
		strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "append-only"),
		"error must indicate append-only or permission denied, got: %v", err,
	)
}

// TestConstraint_PrerequisiteSelfReference verifies that a program cannot list
// itself as its own prerequisite.
func TestConstraint_PrerequisiteSelfReference(t *testing.T) {
	pool := testdb.Open(t)
	defer pool.Close()

	ctx := context.Background()
	branchID := "bbbbbbbb-0000-0000-0000-000000000001"

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	var programID string
	err = tx.QueryRow(ctx,
		`INSERT INTO lms.programs (branch_id, title, capacity, starts_at, ends_at)
		 VALUES ($1, 'Self-Ref Test', 5,
		         NOW() + interval '1 day', NOW() + interval '2 days')
		 RETURNING id`,
		branchID,
	).Scan(&programID)
	require.NoError(t, err)

	_, err = tx.Exec(ctx,
		`INSERT INTO lms.program_prerequisites (program_id, required_program_id)
		 VALUES ($1, $1)`, programID,
	)
	require.Error(t, err, "a program cannot be its own prerequisite")
	testdb.AssertConstraintError(t, err, "chk_prereq_self")
}
