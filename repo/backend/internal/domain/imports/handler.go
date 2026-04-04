// Package imports provides HTTP handlers for the bulk import pipeline.
package imports

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves /imports/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new imports Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all import routes on the given group.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/imports", middlewares...)
	g.GET("", h.ListJobs)
	g.POST("", h.Upload)
	g.GET("/template/:type", h.DownloadTemplate)
	g.GET("/:id", h.GetPreview)
	g.POST("/:id/commit", h.Commit)
	g.POST("/:id/rollback", h.Rollback)
	g.GET("/:id/errors.csv", h.DownloadErrors)
}

// ── Upload ────────────────────────────────────────────────────────────────────

// Upload handles multipart file upload.
// Form fields: import_type (readers|holdings), file (CSV).
// On success returns 202 with the ImportJob (status: preview_ready or failed).
func (h *Handler) Upload(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("imports:create") {
		return &apperr.Forbidden{Action: "upload", Resource: "import"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	importType := c.FormValue("import_type")
	if importType == "" {
		return &apperr.Validation{Field: "import_type", Message: "import_type is required"}
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return &apperr.Validation{Field: "file", Message: "file is required"}
	}

	// Reject suspiciously large files (10 MB limit).
	const maxFileSize = 10 << 20
	if fh.Size > maxFileSize {
		return &apperr.Validation{Field: "file", Message: "file exceeds 10 MB limit"}
	}

	f, err := fh.Open()
	if err != nil {
		return fmt.Errorf("open upload: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return fmt.Errorf("read upload: %w", err)
	}

	result, err := h.service.UploadAndParse(ctx, UploadRequest{
		BranchID:      branchID,
		ActorUserID:   user.User.ID,
		WorkstationID: ctxutil.GetWorkstationID(c),
		ImportType:    importType,
		FileName:      fh.Filename,
		Data:          data,
	})
	if err != nil {
		return err
	}

	status := http.StatusAccepted
	if result.HasErrors {
		status = http.StatusUnprocessableEntity
	}
	return c.JSON(status, result.Job)
}

// ── List jobs ─────────────────────────────────────────────────────────────────

func (h *Handler) ListJobs(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("imports:preview") && !user.HasPermission("imports:create") {
		return &apperr.Forbidden{Action: "list", Resource: "imports"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	p := parsePagination(c)
	result, err := h.service.ListJobs(ctx, branchID, p)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ── Preview ───────────────────────────────────────────────────────────────────

type previewResponse struct {
	Job  *model.ImportJob              `json:"job"`
	Rows model.PageResult[*model.ImportRow] `json:"rows"`
}

func (h *Handler) GetPreview(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("imports:preview") && !user.HasPermission("imports:create") {
		return &apperr.Forbidden{Action: "read", Resource: "import"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	p := parsePagination(c)
	job, rows, err := h.service.GetJobPreview(ctx, c.Param("id"), branchID, p)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, previewResponse{Job: job, Rows: rows})
}

// ── Commit ────────────────────────────────────────────────────────────────────

func (h *Handler) Commit(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("imports:commit") {
		return &apperr.Forbidden{Action: "commit", Resource: "import"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	job, err := h.service.CommitJob(ctx, c.Param("id"), branchID, user.User.ID)
	if err != nil {
		// Return the job alongside the validation error so the client can
		// inspect error details without a second round-trip.
		if job != nil {
			return c.JSON(http.StatusUnprocessableEntity, map[string]any{
				"error":   "commit_failed",
				"detail":  err.Error(),
				"job":     job,
			})
		}
		return err
	}
	return c.JSON(http.StatusOK, job)
}

// ── Rollback ──────────────────────────────────────────────────────────────────

func (h *Handler) Rollback(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("imports:commit") {
		return &apperr.Forbidden{Action: "rollback", Resource: "import"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	job, err := h.service.RollbackJob(ctx, c.Param("id"), branchID, user.User.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, job)
}

// ── Download errors ───────────────────────────────────────────────────────────

func (h *Handler) DownloadErrors(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("imports:preview") && !user.HasPermission("imports:create") {
		return &apperr.Forbidden{Action: "download", Resource: "import errors"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	data, fileName, err := h.service.ErrorCSV(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+fileName+`"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}

// ── Template ──────────────────────────────────────────────────────────────────

func (h *Handler) DownloadTemplate(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("imports:preview") && !user.HasPermission("imports:create") {
		return &apperr.Forbidden{Action: "download", Resource: "import template"}
	}
	_ = ctx

	data, fileName, err := TemplateCSV(c.Param("type"))
	if err != nil {
		return err
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+fileName+`"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", data)
}

// ── Pagination helper ─────────────────────────────────────────────────────────

func parsePagination(c echo.Context) model.Pagination {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	perPage, _ := strconv.Atoi(c.QueryParam("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return model.Pagination{Page: page, PerPage: perPage}
}
