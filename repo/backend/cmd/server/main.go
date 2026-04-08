// Command server is the main entry point for the LMS backend service.
// It loads configuration, connects to PostgreSQL, runs pending migrations,
// registers routes, and starts the HTTP server.
//
// Usage:
//
//	go run ./cmd/server            # from the backend/ directory
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"lms/internal/apierr"
	"lms/internal/audit"
	"lms/internal/config"
	"lms/internal/crypto"
	"lms/internal/db"
	"lms/internal/domain/appeals"
	"lms/internal/domain/circulation"
	"lms/internal/domain/content"
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
	"lms/internal/scheduler"
	"lms/internal/store/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("server terminated with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// --- Configuration ---
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	// Log only safe fields — never log DSN, secrets, or key paths.
	slog.Info("configuration loaded", slog.Any("config", cfg.SafeLogFields()))

	// --- Crypto Key ---
	// Load the AES-256 key if a key file path is configured. If the path is
	// empty, log a warning and continue — Phase 4 will require the key for
	// field-level encryption. The auth flow (Phase 2) does not encrypt reader fields.
	var encryptionKey []byte
	if cfg.Crypto.KeyFile != "" {
		encryptionKey, err = crypto.LoadKey(cfg.Crypto.KeyFile)
		if err != nil {
			return fmt.Errorf("crypto key: %w", err)
		}
		slog.Info("encryption key loaded successfully")
	} else {
		slog.Warn("CRYPTO_KEY_FILE not set — field-level encryption disabled (Phase 4 will require it)")
	}
	// encryptionKey is passed to the reader service for field-level encryption.

	// --- Database ---
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DB.DSN)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	slog.Info("database connection pool established")

	// --- Migrations ---
	if err := runMigrations(cfg.DB.DSN, cfg.Migrate.Path); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	slog.Info("database migrations applied")

	// --- Repositories ---
	userRepo := postgres.NewUserRepo(pool)
	sessionRepo := postgres.NewSessionRepo(pool)
	captchaRepo := postgres.NewCaptchaRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)

	// --- Audit Logger ---
	auditLogger := audit.New(auditRepo)

	// --- Auth Service ---
	authService := users.NewService(
		userRepo,
		sessionRepo,
		captchaRepo,
		auditLogger,
		cfg.Session.InactivityTimeoutSeconds,
	)

	// --- Reader Repository & Service ---
	readerRepo := postgres.NewReaderRepo(pool)
	readerService := readers.NewService(readerRepo, encryptionKey, auditLogger)

	// --- Holdings / Copies Repositories & Service ---
	holdingRepo := postgres.NewHoldingRepo(pool)
	copyRepo := postgres.NewCopyRepo(pool)
	holdingsService := holdings.NewService(holdingRepo, copyRepo, auditLogger)

	// --- Circulation Repository & Service ---
	circulationRepo := postgres.NewCirculationRepo(pool)
	circulationService := circulation.NewService(circulationRepo, copyRepo, auditLogger)

	// --- Stocktake Repository & Service ---
	stocktakeRepo := postgres.NewStocktakeRepo(pool)
	stocktakeService := stocktake.NewService(stocktakeRepo, copyRepo, auditLogger)

	// --- Programs Repository & Service ---
	programRepo := postgres.NewProgramRepo(pool)
	programService := programs.NewService(programRepo, auditLogger)

	// --- Enrollment Repository & Service ---
	enrollmentRepo := postgres.NewEnrollmentRepo(pool)
	enrollmentService := enrollment.NewService(enrollmentRepo, programRepo, readerRepo, auditLogger)

	// --- Content / Moderation / Feedback / Appeals Repositories & Services ---
	contentRepo := postgres.NewContentRepo(pool)
	moderationRepo := postgres.NewModerationRepo(pool)
	feedbackRepo := postgres.NewFeedbackRepo(pool)
	appealsRepo := postgres.NewAppealsRepo(pool)

	moderationService := moderation.NewService(moderationRepo, contentRepo, auditLogger)
	contentService := content.NewService(contentRepo, moderationService, auditLogger)
	feedbackService := feedback.NewService(feedbackRepo, auditLogger)
	appealsService := appeals.NewService(appealsRepo, auditLogger)

	// --- Import Repository & Service ---
	importRepo := postgres.NewImportRepo(pool)
	importService := imports.NewService(importRepo, pool, auditLogger)

	// --- Export Repository & Service ---
	exportRepo := postgres.NewExportRepo(pool)
	exportService := exports.NewService(exportRepo, pool, auditLogger)

	// --- Reports Repository & Service ---
	reportsRepo := postgres.NewReportsRepo(pool)
	reportsService := reports.NewService(reportsRepo, exportRepo, auditLogger)

	// --- HTTP Server ---
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Custom error handler maps apperr types to appropriate HTTP status codes.
	e.HTTPErrorHandler = apierr.ErrorHandler

	// Global middleware (applied to all routes).
	e.Use(appmw.RequestID())
	e.Use(echomw.RecoverWithConfig(echomw.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			slog.Error("panic recovered",
				"error", err,
				"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
			)
			return nil
		},
	}))
	e.Use(echomw.LoggerWithConfig(echomw.LoggerConfig{
		// Structured JSON logging — no sensitive query params logged.
		Format: `{"time":"${time_rfc3339}","level":"INFO","request_id":"${id}","method":"${method}","uri":"${uri}","status":${status},"latency_ms":${latency_ms}}` + "\n",
	}))

	// Session middleware for protected route groups.
	requireAuth := appmw.RequireAuth(sessionRepo, userRepo, cfg.Session.InactivityTimeoutSeconds)

	// --- Nightly scheduler ---
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	defer schedulerCancel()

	nightly := scheduler.New(scheduler.Job{
		Name: "reports.daily_preaggregation",
		Run: func(ctx context.Context, date time.Time) error {
			targetDay := date.AddDate(0, 0, -1).Truncate(24 * time.Hour)
			branchIDs, err := listBranchIDs(ctx, pool)
			if err != nil {
				return err
			}

			for _, branchID := range branchIDs {
				if _, err := reportsService.RecalculateAggregates(ctx, reports.RecalcRequest{
					BranchID:    branchID,
					From:        targetDay,
					To:          targetDay,
					ActorUserID: "",
				}); err != nil {
					slog.Error("nightly report aggregation failed", "branch_id", branchID, "date", targetDay.Format("2006-01-02"), "error", err)
				}
			}

			if _, err := reportsService.RecalculateAggregates(ctx, reports.RecalcRequest{
				BranchID:    "",
				From:        targetDay,
				To:          targetDay,
				ActorUserID: "",
			}); err != nil {
				slog.Error("nightly all-branches report aggregation failed", "date", targetDay.Format("2006-01-02"), "error", err)
			}

			return nil
		},
	})
	go nightly.Start(schedulerCtx)

	// --- Routes ---
	registerRoutes(e, pool, authService, readerService, holdingsService, circulationService, stocktakeService,
		programService, enrollmentService, importService, exportService,
		contentService, moderationService, feedbackService, appealsService, reportsService,
		userRepo, sessionRepo, auditLogger, requireAuth)

	// --- Graceful shutdown ---
	serverAddr := cfg.Addr()
	slog.Info("starting HTTP server", "addr", serverAddr)

	go func() {
		if err := e.Start(serverAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutdown signal received, draining connections")
	schedulerCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	slog.Info("server stopped cleanly")
	return nil
}

// registerRoutes wires all HTTP routes to their handlers.
func registerRoutes(
	e *echo.Echo,
	pool *db.Pool,
	authService *users.Service,
	readerService *readers.Service,
	holdingsService *holdings.Service,
	circulationService *circulation.Service,
	stocktakeService *stocktake.Service,
	programService *programs.Service,
	enrollmentService *enrollment.Service,
	importService *imports.Service,
	exportService *exports.Service,
	contentService *content.Service,
	moderationService *moderation.Service,
	feedbackService *feedback.Service,
	appealsService *appeals.Service,
	reportsService *reports.Service,
	userRepo *postgres.UserRepo,
	sessionRepo *postgres.SessionRepo,
	auditLogger *audit.Logger,
	requireAuth echo.MiddlewareFunc,
) {
	api := e.Group("/api/v1")

	// Liveness — no database check. Safe to call before DB is ready.
	api.GET("/health", health.LiveHandler)

	// Readiness — pings the database. Use this to verify DB connectivity.
	api.GET("/ready", health.ReadyHandler(pool))

	// ── Auth routes ──────────────────────────────────────────────────────────
	authHandler := users.NewHandlerWithRepo(authService, userRepo, auditLogger)
	authHandler.RegisterRoutes(api, requireAuth)

	branchScopeMW := appmw.BranchScope(userRepo)

	// ── User management routes ────────────────────────────────────────────────
	authHandler.RegisterUserRoutes(api, requireAuth, branchScopeMW)

	// ── Readers routes ───────────────────────────────────────────────────────
	readersHandler := readers.NewHandler(readerService, auditLogger)
	readersHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Holdings + Copies routes ─────────────────────────────────────────────
	holdingsHandler := holdings.NewHandler(holdingsService)
	holdingsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Circulation routes ───────────────────────────────────────────────────
	circulationHandler := circulation.NewHandler(circulationService)
	circulationHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Stocktake routes ─────────────────────────────────────────────────────
	stocktakeHandler := stocktake.NewHandler(stocktakeService)
	stocktakeHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Programs routes ───────────────────────────────────────────────────────
	programsHandler := programs.NewHandler(programService)
	programsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Enrollment routes ─────────────────────────────────────────────────────
	enrollmentHandler := enrollment.NewHandler(enrollmentService)
	enrollmentHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Imports routes ────────────────────────────────────────────────────────
	importsHandler := imports.NewHandler(importService)
	importsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Exports routes ────────────────────────────────────────────────────────
	exportsHandler := exports.NewHandler(exportService)
	exportsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Content routes ────────────────────────────────────────────────────────
	contentHandler := content.NewHandler(contentService)
	contentHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Moderation routes ─────────────────────────────────────────────────────
	moderationHandler := moderation.NewHandler(moderationService)
	moderationHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Feedback routes ───────────────────────────────────────────────────────
	feedbackHandler := feedback.NewHandler(feedbackService)
	feedbackHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Appeals routes ────────────────────────────────────────────────────────
	appealsHandler := appeals.NewHandler(appealsService)
	appealsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Reports routes ────────────────────────────────────────────────────────
	reportsHandler := reports.NewHandler(reportsService)
	reportsHandler.RegisterRoutes(api, requireAuth, branchScopeMW)

	// ── Protected route group placeholder ────────────────────────────────────
	_ = api.Group("/protected", requireAuth)
}

// runMigrations applies pending SQL migrations from the given directory path.
func runMigrations(dsn, migrationsPath string) error {
	return runMigrationsInternal(dsn, migrationsPath)
}

func listBranchIDs(ctx context.Context, pool *db.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT id::text FROM lms.branches ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
