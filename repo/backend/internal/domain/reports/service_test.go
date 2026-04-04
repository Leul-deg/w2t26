package reports_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	auditpkg "lms/internal/audit"
	"lms/internal/apperr"
	domainaudit "lms/internal/domain/audit"
	"lms/internal/ctxutil"
	"lms/internal/domain/exports"
	"lms/internal/domain/reports"
	"lms/internal/model"
)

type stubReportsRepo struct {
	defs               []*model.ReportDefinition
	defByID            map[string]*model.ReportDefinition
	rows               []map[string]any
	aggregates         []*model.ReportAggregate
	lastBranchID       string
	lastQueryTemplate  string
	lastFilters        map[string]string
	upsertedAggregates []*model.ReportAggregate
}

func newStubReportsRepo() *stubReportsRepo {
	def := &model.ReportDefinition{
		ID:            "def-001",
		Name:          "program_utilization",
		QueryTemplate: "utilization",
		IsActive:      true,
	}
	return &stubReportsRepo{
		defs:    []*model.ReportDefinition{def},
		defByID: map[string]*model.ReportDefinition{def.ID: def},
		rows: []map[string]any{
			{"copy_status": "available", "copy_count": 10},
		},
		aggregates: []*model.ReportAggregate{},
	}
}

func (r *stubReportsRepo) ListDefinitions(_ context.Context) ([]*model.ReportDefinition, error) {
	return r.defs, nil
}

func (r *stubReportsRepo) GetDefinition(_ context.Context, id string) (*model.ReportDefinition, error) {
	def, ok := r.defByID[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return def, nil
}

func (r *stubReportsRepo) GetAggregate(_ context.Context, definitionID, branchID, periodStart, periodEnd string) (*model.ReportAggregate, error) {
	for _, agg := range r.aggregates {
		if agg.ReportDefinitionID == definitionID && agg.PeriodStart == periodStart && agg.PeriodEnd == periodEnd {
			return agg, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (r *stubReportsRepo) UpsertAggregate(_ context.Context, agg *model.ReportAggregate) error {
	if agg.ID == "" {
		agg.ID = fmt.Sprintf("agg-%03d", len(r.upsertedAggregates)+1)
	}
	r.upsertedAggregates = append(r.upsertedAggregates, agg)
	return nil
}

func (r *stubReportsRepo) ListAggregates(_ context.Context, branchID, definitionID string, from, to time.Time) ([]*model.ReportAggregate, error) {
	return r.aggregates, nil
}

func (r *stubReportsRepo) RunLiveQuery(_ context.Context, branchID, queryTemplate string, filters map[string]string, from, to time.Time) ([]map[string]any, error) {
	r.lastBranchID = branchID
	r.lastQueryTemplate = queryTemplate
	r.lastFilters = filters
	return r.rows, nil
}

type stubExportRepo struct {
	created   []*model.ExportJob
	finalised []struct {
		id       string
		rowCount int
		fileName string
	}
}

func (r *stubExportRepo) Create(_ context.Context, job *model.ExportJob) error {
	if job.ID == "" {
		job.ID = fmt.Sprintf("job-%03d", len(r.created)+1)
	}
	now := time.Now().UTC()
	job.ExportedAt = now
	r.created = append(r.created, job)
	return nil
}

func (r *stubExportRepo) Finalise(_ context.Context, id string, rowCount int, fileName string) error {
	r.finalised = append(r.finalised, struct {
		id       string
		rowCount int
		fileName string
	}{id: id, rowCount: rowCount, fileName: fileName})
	return nil
}

func (r *stubExportRepo) List(_ context.Context, branchID string, p model.Pagination) (model.PageResult[*model.ExportJob], error) {
	return model.NewPageResult([]*model.ExportJob{}, 0, p), nil
}

var _ exports.Repository = (*stubExportRepo)(nil)

type captureAuditRepo struct {
	events []*model.AuditEvent
}

func (r *captureAuditRepo) Insert(_ context.Context, e *model.AuditEvent) error {
	r.events = append(r.events, e)
	return nil
}

func (r *captureAuditRepo) List(_ context.Context, _ domainaudit.AuditFilter, p model.Pagination) (model.PageResult[*model.AuditEvent], error) {
	return model.NewPageResult([]*model.AuditEvent{}, 0, p), nil
}

func TestRunReport_PassesFiltersAndReturnsRows(t *testing.T) {
	repo := newStubReportsRepo()
	svc := reports.NewService(repo, &stubExportRepo{}, nil)

	result, err := svc.RunReport(context.Background(), reports.RunRequest{
		BranchID:     "branch-001",
		DefinitionID: "def-001",
		From:         time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		To:           time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		Filters:      map[string]string{"category": "workshop"},
		ActorUserID:  "admin-001",
	})
	if err != nil {
		t.Fatalf("RunReport: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
	}
	if repo.lastBranchID != "branch-001" {
		t.Fatalf("expected branch-001, got %q", repo.lastBranchID)
	}
	if repo.lastQueryTemplate != "utilization" {
		t.Fatalf("expected utilization query template, got %q", repo.lastQueryTemplate)
	}
	if repo.lastFilters["category"] != "workshop" {
		t.Fatalf("expected category filter to pass through, got %#v", repo.lastFilters)
	}
}

func TestRecalculateAggregates_ComputesDailyAggregates(t *testing.T) {
	repo := newStubReportsRepo()
	svc := reports.NewService(repo, &stubExportRepo{}, nil)

	count, err := svc.RecalculateAggregates(context.Background(), reports.RecalcRequest{
		BranchID:     "branch-001",
		DefinitionID: "def-001",
		From:         time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		To:           time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RecalculateAggregates: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 computed aggregates, got %d", count)
	}
	if len(repo.upsertedAggregates) != 2 {
		t.Fatalf("expected 2 upserted aggregates, got %d", len(repo.upsertedAggregates))
	}
}

func TestExportReport_CreatesAuditRecord(t *testing.T) {
	repo := newStubReportsRepo()
	exportRepo := &stubExportRepo{}
	auditRepo := &captureAuditRepo{}
	logger := auditpkg.New(auditRepo)
	svc := reports.NewService(repo, exportRepo, logger)

	result, err := svc.ExportReport(context.Background(), reports.ExportReportRequest{
		BranchID:     "branch-001",
		DefinitionID: "def-001",
		From:         time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		To:           time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		ActorUserID:  "admin-001",
	})
	if err != nil {
		t.Fatalf("ExportReport: %v", err)
	}
	if result.Job == nil || result.Job.ID == "" {
		t.Fatal("expected export job to be created")
	}
	if len(exportRepo.created) != 1 || len(exportRepo.finalised) != 1 {
		t.Fatalf("expected create+finalise export job, got created=%d finalised=%d", len(exportRepo.created), len(exportRepo.finalised))
	}
	if len(auditRepo.events) == 0 {
		t.Fatal("expected export audit event")
	}
	if auditRepo.events[len(auditRepo.events)-1].EventType != model.AuditExportCreated {
		t.Fatalf("expected audit event %q, got %q", model.AuditExportCreated, auditRepo.events[len(auditRepo.events)-1].EventType)
	}
}

func TestHandler_ListDefinitions_RequiresPermission(t *testing.T) {
	h := reports.NewHandler(nil)
	e := echo.New()
	e.GET("/reports/definitions", h.ListDefinitions)

	req := httptest.NewRequest(http.MethodGet, "/reports/definitions", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	user := &model.UserWithRoles{User: &model.User{ID: "u1"}, Permissions: []string{"readers:read"}}
	ctx := ctxutil.SetUser(c.Request().Context(), user)
	c.SetRequest(c.Request().WithContext(ctx))

	err := h.ListDefinitions(c)
	if err == nil {
		t.Fatal("expected forbidden error")
	}
	if _, ok := err.(*apperr.Forbidden); !ok {
		t.Fatalf("expected forbidden error, got %T", err)
	}
}

func TestRunReport_EmptyBranchID_ReturnsValidation(t *testing.T) {
	repo := newStubReportsRepo()
	svc := reports.NewService(repo, &stubExportRepo{}, nil)

	_, err := svc.RunReport(context.Background(), reports.RunRequest{
		BranchID:     "", // deliberately empty
		DefinitionID: "def-001",
		From:         time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		To:           time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		ActorUserID:  "admin-001",
	})
	if err == nil {
		t.Fatal("expected validation error for empty BranchID, got nil")
	}
	var ve *apperr.Validation
	if verr, ok := err.(*apperr.Validation); !ok {
		t.Fatalf("expected *apperr.Validation, got %T: %v", err, err)
	} else {
		ve = verr
	}
	if ve.Field != "branch_id" {
		t.Fatalf("expected field=branch_id, got %q", ve.Field)
	}
}

func TestExportReport_EmptyBranchID_ReturnsValidation(t *testing.T) {
	repo := newStubReportsRepo()
	svc := reports.NewService(repo, &stubExportRepo{}, nil)

	_, err := svc.ExportReport(context.Background(), reports.ExportReportRequest{
		BranchID:     "", // deliberately empty
		DefinitionID: "def-001",
		From:         time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		To:           time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		ActorUserID:  "admin-001",
	})
	if err == nil {
		t.Fatal("expected validation error for empty BranchID, got nil")
	}
	if _, ok := err.(*apperr.Validation); !ok {
		t.Fatalf("expected *apperr.Validation, got %T: %v", err, err)
	}
}

func TestHandler_Recalculate_RequiresAdminPermission(t *testing.T) {
	h := reports.NewHandler(nil)
	e := echo.New()
	e.POST("/reports/recalculate", h.Recalculate)

	req := httptest.NewRequest(http.MethodPost, "/reports/recalculate",
		strings.NewReader(`{"from":"2026-03-01","to":"2026-03-31"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	user := &model.UserWithRoles{User: &model.User{ID: "u1"}, Permissions: []string{"reports:read"}}
	ctx := ctxutil.SetUser(c.Request().Context(), user)
	c.SetRequest(c.Request().WithContext(ctx))

	err := h.Recalculate(c)
	if err == nil {
		t.Fatal("expected forbidden error")
	}
	if _, ok := err.(*apperr.Forbidden); !ok {
		t.Fatalf("expected forbidden error, got %T", err)
	}
}
