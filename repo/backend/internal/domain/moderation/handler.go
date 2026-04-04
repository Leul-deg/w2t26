// Package moderation provides HTTP handlers for the content moderation queue.
package moderation

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves /moderation/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new moderation Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires all moderation routes.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/moderation", middlewares...)
	g.GET("/queue", h.ListQueue)
	g.GET("/items/:id", h.GetItem)
	g.POST("/items/:id/assign", h.Assign)
	g.POST("/items/:id/decide", h.Decide)
}

// ── ListQueue ─────────────────────────────────────────────────────────────────

func (h *Handler) ListQueue(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:moderate") {
		return &apperr.Forbidden{Action: "list", Resource: "moderation_queue"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	f := ModerationFilter{}
	if s := c.QueryParam("status"); s != "" {
		f.Status = &s
	} else {
		// Default: show pending and in_review items.
		pending := "pending"
		f.Status = &pending
	}
	if a := c.QueryParam("assigned_to"); a != "" {
		f.AssignedTo = &a
	}

	result, err := h.service.ListQueue(ctx, branchID, f, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ── GetItem ───────────────────────────────────────────────────────────────────

func (h *Handler) GetItem(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:moderate") {
		return &apperr.Forbidden{Action: "get", Resource: "moderation_item"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	item, content, err := h.service.GetItem(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"item":    item,
		"content": content,
	})
}

// ── Assign ────────────────────────────────────────────────────────────────────

func (h *Handler) Assign(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:moderate") {
		return &apperr.Forbidden{Action: "assign", Resource: "moderation_item"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	item, err := h.service.Assign(ctx, c.Param("id"), branchID, user.User.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// ── Decide ────────────────────────────────────────────────────────────────────

type decideReq struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func (h *Handler) Decide(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("content:moderate") {
		return &apperr.Forbidden{Action: "decide", Resource: "moderation_item"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var req decideReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Message: "invalid request body"}
	}

	item, err := h.service.Decide(ctx, DecideRequest{
		ItemID:      c.Param("id"),
		BranchID:    branchID,
		Decision:    req.Decision,
		Reason:      req.Reason,
		ActorUserID: user.User.ID,
	})
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
