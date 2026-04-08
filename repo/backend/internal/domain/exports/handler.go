// Package exports provides HTTP handlers for the audited export pipeline.
package exports

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves /exports/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new exports Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires all export routes onto the given group.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/exports", middlewares...)
	// List audit records.
	g.GET("", h.ListJobs)
	// Trigger a new export — type in query param avoids a secondary path segment.
	g.POST("/readers", h.ExportReaders)
	g.POST("/holdings", h.ExportHoldings)
}

// ── List export jobs ──────────────────────────────────────────────────────────

func (h *Handler) ListJobs(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("exports:create") {
		return &apperr.Forbidden{Action: "list", Resource: "exports"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	p := parsePagination(c)
	result, err := h.service.ListJobs(ctx, branchID, p)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ── Export readers ────────────────────────────────────────────────────────────

func (h *Handler) ExportReaders(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("exports:create") {
		return &apperr.Forbidden{Action: "export", Resource: "readers"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	result, err := h.service.Export(ctx, ExportRequest{
		BranchID:      branchID,
		ActorUserID:   user.User.ID,
		WorkstationID: ctxutil.GetWorkstationID(c),
		ExportType:    "readers",
		Format:        c.QueryParam("format"),
	})
	if err != nil {
		return err
	}
	c.Response().Header().Set(echo.HeaderContentDisposition,
		`attachment; filename="`+result.FileName+`"`)
	c.Response().Header().Set("X-Export-Job-ID", result.Job.ID)
	return c.Blob(http.StatusOK, result.ContentType, result.Data)
}

// ── Export holdings ───────────────────────────────────────────────────────────

func (h *Handler) ExportHoldings(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("exports:create") {
		return &apperr.Forbidden{Action: "export", Resource: "holdings"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	result, err := h.service.Export(ctx, ExportRequest{
		BranchID:      branchID,
		ActorUserID:   user.User.ID,
		WorkstationID: ctxutil.GetWorkstationID(c),
		ExportType:    "holdings",
		Format:        c.QueryParam("format"),
	})
	if err != nil {
		return err
	}
	c.Response().Header().Set(echo.HeaderContentDisposition,
		`attachment; filename="`+result.FileName+`"`)
	c.Response().Header().Set("X-Export-Job-ID", result.Job.ID)
	return c.Blob(http.StatusOK, result.ContentType, result.Data)
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
