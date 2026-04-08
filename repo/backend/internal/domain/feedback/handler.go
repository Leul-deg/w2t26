// Package feedback provides HTTP handlers for reader feedback management.
package feedback

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves /feedback/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new feedback Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires all feedback routes.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/feedback", middlewares...)
	g.GET("", h.List)
	g.POST("", h.Submit)
	g.GET("/tags", h.ListTags)
	g.GET("/:id", h.Get)
	g.POST("/:id/moderate", h.Moderate)
}

// ── List ──────────────────────────────────────────────────────────────────────

func (h *Handler) List(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("feedback:read") {
		return &apperr.Forbidden{Action: "list", Resource: "feedback"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	f := FeedbackFilter{}
	if s := c.QueryParam("status"); s != "" {
		f.Status = &s
	}
	if t := c.QueryParam("target_type"); t != "" {
		f.TargetType = &t
	}
	if id := c.QueryParam("target_id"); id != "" {
		f.TargetID = &id
	}

	result, err := h.service.List(ctx, branchID, f, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ── Submit ────────────────────────────────────────────────────────────────────

type submitFeedbackReq struct {
	ReaderID   string   `json:"reader_id"`
	TargetType string   `json:"target_type"`
	TargetID   string   `json:"target_id"`
	Rating     *int     `json:"rating"`
	Comment    *string  `json:"comment"`
	Tags       []string `json:"tags"`
}

func (h *Handler) Submit(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("feedback:submit") {
		return &apperr.Forbidden{Action: "submit", Resource: "feedback"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var req submitFeedbackReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Message: "invalid request body"}
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	item, err := h.service.Submit(ctx, SubmitRequest{
		BranchID:    branchID,
		ReaderID:    req.ReaderID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		Rating:      req.Rating,
		Comment:     req.Comment,
		TagNames:    req.Tags,
		ActorUserID: user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, item)
}

// ── ListTags ──────────────────────────────────────────────────────────────────

func (h *Handler) ListTags(c echo.Context) error {
	ctx := c.Request().Context()
	if _, ok := ctxutil.GetUser(ctx); !ok {
		return &apperr.Unauthorized{}
	}
	tags, err := h.service.ListTags(ctx)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tags)
}

// ── Get ───────────────────────────────────────────────────────────────────────

func (h *Handler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("feedback:read") {
		return &apperr.Forbidden{Action: "get", Resource: "feedback"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	item, err := h.service.Get(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// ── Moderate ──────────────────────────────────────────────────────────────────

type moderateFeedbackReq struct {
	Status string `json:"status"`
}

func (h *Handler) Moderate(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("feedback:moderate") {
		return &apperr.Forbidden{Action: "moderate", Resource: "feedback"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var req moderateFeedbackReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Message: "invalid request body"}
	}

	item, err := h.service.Moderate(ctx, ModerateRequest{
		ID:          c.Param("id"),
		BranchID:    branchID,
		Status:      req.Status,
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
