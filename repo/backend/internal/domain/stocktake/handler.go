// Package stocktake provides HTTP handlers for inventory stocktake sessions.
package stocktake

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves /stocktake/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new stocktake Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all stocktake routes.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/stocktake", middlewares...)
	g.GET("", h.ListSessions)
	g.POST("", h.CreateSession)
	g.GET("/:id", h.GetSession)
	g.PATCH("/:id/status", h.CloseSession)
	g.GET("/:id/findings", h.ListFindings)
	g.POST("/:id/scan", h.RecordScan)
	g.GET("/:id/variances", h.GetVariances)
}

// ListSessions handles GET /stocktake.
func (h *Handler) ListSessions(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("stocktake:read") {
		return &apperr.Forbidden{Action: "list", Resource: "stocktake sessions"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	p := parsePagination(c)
	result, err := h.service.ListSessions(ctx, branchID, p)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

type createSessionReq struct {
	BranchID string  `json:"branch_id"`
	Name     string  `json:"name"`
	Notes    *string `json:"notes"`
}

// CreateSession handles POST /stocktake.
func (h *Handler) CreateSession(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("stocktake:write") {
		return &apperr.Forbidden{Action: "create", Resource: "stocktake session"}
	}

	var req createSessionReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.BranchID == "" {
		req.BranchID, _ = ctxutil.GetBranchID(ctx)
	}

	session, err := h.service.CreateSession(ctx, req.BranchID, user.User.ID, req.Name, req.Notes)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, session)
}

// GetSession handles GET /stocktake/:id.
func (h *Handler) GetSession(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("stocktake:read") {
		return &apperr.Forbidden{Action: "read", Resource: "stocktake session"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	session, err := h.service.GetSession(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, session)
}

type closeSessionReq struct {
	Status string `json:"status"` // "closed" or "cancelled"
}

// CloseSession handles PATCH /stocktake/:id/status.
func (h *Handler) CloseSession(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("stocktake:write") {
		return &apperr.Forbidden{Action: "close", Resource: "stocktake session"}
	}

	var req closeSessionReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.Status == "" {
		return &apperr.Validation{Field: "status", Message: "status is required (closed or cancelled)"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	if err := h.service.CloseSession(ctx, c.Param("id"), branchID, user.User.ID, req.Status); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"status": req.Status})
}

// ListFindings handles GET /stocktake/:id/findings.
func (h *Handler) ListFindings(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("stocktake:read") {
		return &apperr.Forbidden{Action: "list", Resource: "stocktake findings"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	p := parsePagination(c)
	result, err := h.service.ListFindings(ctx, c.Param("id"), branchID, p)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

type recordScanReq struct {
	Barcode     string  `json:"barcode"`
	FindingType *string `json:"finding_type"` // optional override: "damaged"
	Notes       *string `json:"notes"`
}

// RecordScan handles POST /stocktake/:id/scan.
func (h *Handler) RecordScan(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("stocktake:write") {
		return &apperr.Forbidden{Action: "scan", Resource: "stocktake session"}
	}

	var req recordScanReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.Barcode == "" {
		return &apperr.Validation{Field: "barcode", Message: "barcode is required"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	finding, err := h.service.RecordScan(ctx, c.Param("id"), branchID, user.User.ID, req.Barcode, req.FindingType, req.Notes)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, finding)
}

// GetVariances handles GET /stocktake/:id/variances.
func (h *Handler) GetVariances(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("stocktake:read") {
		return &apperr.Forbidden{Action: "read", Resource: "stocktake variances"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	variances, err := h.service.GetVariances(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"session_id": c.Param("id"),
		"variances":  variances,
		"count":      len(variances),
	})
}

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
