package apitests

// complete_app_test.go provides a fully-wired Echo test application with every
// domain service registered. New CRUD API tests import newCompleteTestApp() to
// get a single app instance that covers all routes.

import (
	"testing"

	"github.com/labstack/echo/v4"

	"lms/internal/apierr"
	auditpkg "lms/internal/audit"
	"lms/internal/domain/appeals"
	"lms/internal/domain/circulation"
	"lms/internal/domain/content"
	copies "lms/internal/domain/copies"
	"lms/internal/domain/enrollment"
	"lms/internal/domain/exports"
	"lms/internal/domain/feedback"
	"lms/internal/domain/holdings"
	"lms/internal/domain/imports"
	"lms/internal/domain/moderation"
	"lms/internal/domain/programs"
	"lms/internal/domain/readers"
	"lms/internal/domain/reports"
	"lms/internal/domain/stocktake"
	"lms/internal/domain/users"
	"lms/internal/health"
	appmw "lms/internal/middleware"
	"lms/internal/store/postgres"
	"lms/tests/testdb"
)

// completeTestApp wires ALL domain services and registers ALL routes, mirroring
// the production server wiring in cmd/server/main.go. Use this for end-to-end
// CRUD tests that exercise the full request path.
type completeTestApp struct {
	*testApp
}

// newCompleteTestApp creates and returns a fully-wired Echo test application.
func newCompleteTestApp(t *testing.T) *completeTestApp {
	t.Helper()
	pool := testdb.Open(t)
	t.Cleanup(func() { pool.Close() })

	// ── Core repos ───────────────────────────────────────────────────────────
	userRepo := postgres.NewUserRepo(pool)
	sessionRepo := postgres.NewSessionRepo(pool)
	captchaRepo := postgres.NewCaptchaRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)
	auditLogger := auditpkg.New(auditRepo)

	// ── Domain repos ─────────────────────────────────────────────────────────
	readerRepo := postgres.NewReaderRepo(pool)
	holdingRepo := postgres.NewHoldingRepo(pool)
	copyRepo := postgres.NewCopyRepo(pool)
	circulationRepo := postgres.NewCirculationRepo(pool)
	stocktakeRepo := postgres.NewStocktakeRepo(pool)
	programRepo := postgres.NewProgramRepo(pool)
	enrollmentRepo := postgres.NewEnrollmentRepo(pool)
	contentRepo := postgres.NewContentRepo(pool)
	moderationRepo := postgres.NewModerationRepo(pool)
	feedbackRepo := postgres.NewFeedbackRepo(pool)
	appealsRepo := postgres.NewAppealsRepo(pool)
	importRepo := postgres.NewImportRepo(pool)
	exportRepo := postgres.NewExportRepo(pool)
	reportsRepo := postgres.NewReportsRepo(pool)

	// ── Services ─────────────────────────────────────────────────────────────
	authService := users.NewService(userRepo, sessionRepo, captchaRepo, auditLogger, 1800)
	readerService := readers.NewService(readerRepo, nil, auditLogger)
	holdingsService := holdings.NewService(holdingRepo, copyRepo, auditLogger)
	circulationService := circulation.NewService(circulationRepo, copies.Repository(copyRepo), auditLogger)
	stocktakeService := stocktake.NewService(stocktakeRepo, copies.Repository(copyRepo), auditLogger)
	programService := programs.NewService(programRepo, auditLogger)
	enrollmentService := enrollment.NewService(enrollmentRepo, programRepo, readerRepo, auditLogger)
	moderationService := moderation.NewService(moderationRepo, contentRepo, auditLogger)
	contentService := content.NewService(contentRepo, moderationService, auditLogger)
	feedbackService := feedback.NewService(feedbackRepo, auditLogger)
	appealsService := appeals.NewService(appealsRepo, auditLogger)
	importService := imports.NewService(importRepo, pool, auditLogger)
	exportService := exports.NewService(exportRepo, pool, auditLogger)
	reportsService := reports.NewService(reportsRepo, exportRepo, auditLogger)

	// ── Echo ─────────────────────────────────────────────────────────────────
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = apierr.ErrorHandler

	requireAuth := appmw.RequireAuth(sessionRepo, userRepo, 1800)
	branchScopeMW := appmw.BranchScope(userRepo)

	api := e.Group("/api/v1")

	// Health (no auth).
	api.GET("/health", health.LiveHandler)
	api.GET("/ready", health.ReadyHandler(pool))

	// Auth + user management.
	authHandler := users.NewHandlerWithRepo(authService, userRepo, auditLogger)
	authHandler.RegisterRoutes(api, requireAuth)
	authHandler.RegisterUserRoutes(api, requireAuth, branchScopeMW)

	// Readers.
	readers.NewHandler(readerService, auditLogger).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Holdings + copies.
	holdings.NewHandler(holdingsService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Circulation.
	circulation.NewHandler(circulationService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Stocktake.
	stocktake.NewHandler(stocktakeService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Programs.
	programs.NewHandler(programService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Enrollment.
	enrollment.NewHandler(enrollmentService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Content.
	content.NewHandler(contentService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Moderation.
	moderation.NewHandler(moderationService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Feedback.
	feedback.NewHandler(feedbackService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Appeals.
	appeals.NewHandler(appealsService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Imports.
	imports.NewHandler(importService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Exports.
	exports.NewHandler(exportService).RegisterRoutes(api, requireAuth, branchScopeMW)

	// Reports.
	reports.NewHandler(reportsService).RegisterRoutes(api, requireAuth, branchScopeMW)

	base := &testApp{
		e:           e,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		captchaRepo: captchaRepo,
		auditRepo:   auditRepo,
		authService: authService,
		auditLogger: auditLogger,
	}
	return &completeTestApp{testApp: base}
}
