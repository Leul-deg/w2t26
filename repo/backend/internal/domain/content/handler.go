// Package content provides HTTP handlers for governed content management.
package content

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves /content/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new content Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires all content routes.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/content", middlewares...)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Update)
	g.POST("/:id/submit", h.Submit)
	g.POST("/:id/retract", h.Retract)
	g.POST("/:id/publish", h.Publish)
	g.POST("/:id/archive", h.Archive)
}

// ── List ──────────────────────────────────────────────────────────────────────

func (h *Handler) List(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:read") {
		return &apperr.Forbidden{Action: "list", Resource: "content"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	f := ContentFilter{}
	if s := c.QueryParam("status"); s != "" {
		f.Status = &s
	}
	if ct := c.QueryParam("content_type"); ct != "" {
		f.ContentType = &ct
	}
	result, err := h.service.List(ctx, branchID, f, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ── Create ────────────────────────────────────────────────────────────────────

type createContentReq struct {
	Title       string  `json:"title"`
	ContentType string  `json:"content_type"`
	Body        *string `json:"body"`
	FileName    *string `json:"file_name"`
}

func (h *Handler) Create(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:submit") {
		return &apperr.Forbidden{Action: "create", Resource: "content"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var req createContentReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Message: "invalid request body"}
	}
	item, err := h.service.Create(ctx, CreateRequest{
		BranchID:    branchID,
		Title:       req.Title,
		ContentType: req.ContentType,
		Body:        req.Body,
		FileName:    req.FileName,
		ActorUserID: user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, item)
}

// ── Get ───────────────────────────────────────────────────────────────────────

func (h *Handler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:read") {
		return &apperr.Forbidden{Action: "get", Resource: "content"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	item, err := h.service.Get(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// ── Update ────────────────────────────────────────────────────────────────────

type updateContentReq struct {
	Title    *string `json:"title"`
	Body     *string `json:"body"`
	FileName *string `json:"file_name"`
}

func (h *Handler) Update(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:submit") {
		return &apperr.Forbidden{Action: "update", Resource: "content"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var req updateContentReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Message: "invalid request body"}
	}
	item, err := h.service.Update(ctx, UpdateRequest{
		ID:          c.Param("id"),
		BranchID:    branchID,
		Title:       req.Title,
		Body:        req.Body,
		FileName:    req.FileName,
		ActorUserID: user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// ── Submit ────────────────────────────────────────────────────────────────────

func (h *Handler) Submit(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:submit") {
		return &apperr.Forbidden{Action: "submit", Resource: "content"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	item, err := h.service.Submit(ctx, c.Param("id"), branchID, user.User.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// ── Retract ───────────────────────────────────────────────────────────────────

func (h *Handler) Retract(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:submit") {
		return &apperr.Forbidden{Action: "retract", Resource: "content"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	item, err := h.service.Retract(ctx, c.Param("id"), branchID, user.User.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// ── Publish ───────────────────────────────────────────────────────────────────

func (h *Handler) Publish(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:publish") {
		return &apperr.Forbidden{Action: "publish", Resource: "content"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	item, err := h.service.Publish(ctx, c.Param("id"), branchID, user.User.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// ── Archive ───────────────────────────────────────────────────────────────────

func (h *Handler) Archive(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:publish") {
		return &apperr.Forbidden{Action: "archive", Resource: "content"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	item, err := h.service.Archive(ctx, c.Param("id"), branchID, user.User.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// ── helpers ───────────────────────────────────────────────────────────────────

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
