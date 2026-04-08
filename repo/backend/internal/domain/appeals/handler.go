// Package appeals provides HTTP handlers for appeal submission and arbitration.
package appeals

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves /appeals/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new appeals Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires all appeal routes.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/appeals", middlewares...)
	g.GET("", h.List)
	g.POST("", h.Submit)
	g.GET("/:id", h.Get)
	g.POST("/:id/review", h.Review)
	g.POST("/:id/arbitrate", h.Arbitrate)
}

// ── List ──────────────────────────────────────────────────────────────────────

func (h *Handler) List(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("appeals:read") {
		return &apperr.Forbidden{Action: "list", Resource: "appeals"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	f := AppealFilter{}
	if s := c.QueryParam("status"); s != "" {
		f.Status = &s
	}
	if t := c.QueryParam("appeal_type"); t != "" {
		f.AppealType = &t
	}
	if r := c.QueryParam("reader_id"); r != "" {
		f.ReaderID = &r
	}

	result, err := h.service.List(ctx, branchID, f, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ── Submit ────────────────────────────────────────────────────────────────────

type submitAppealReq struct {
	ReaderID   string  `json:"reader_id"`
	AppealType string  `json:"appeal_type"`
	TargetType *string `json:"target_type"`
	TargetID   *string `json:"target_id"`
	Reason     string  `json:"reason"`
}

func (h *Handler) Submit(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("appeals:submit") {
		return &apperr.Forbidden{Action: "submit", Resource: "appeal"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var req submitAppealReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Message: "invalid request body"}
	}

	appeal, err := h.service.Submit(ctx, SubmitRequest{
		BranchID:    branchID,
		ReaderID:    req.ReaderID,
		AppealType:  req.AppealType,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		Reason:      req.Reason,
		ActorUserID: user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, appeal)
}

// ── Get ───────────────────────────────────────────────────────────────────────

func (h *Handler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("appeals:read") {
		return &apperr.Forbidden{Action: "get", Resource: "appeal"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	appeal, arb, err := h.service.Get(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"appeal":      appeal,
		"arbitration": arb,
	})
}

// ── Review ────────────────────────────────────────────────────────────────────

func (h *Handler) Review(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("appeals:read") {
		return &apperr.Forbidden{Action: "review", Resource: "appeal"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	appeal, err := h.service.Review(ctx, c.Param("id"), branchID, user.User.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, appeal)
}

// ── Arbitrate ─────────────────────────────────────────────────────────────────

type arbitrateReq struct {
	Decision      string `json:"decision"`
	DecisionNotes string `json:"decision_notes"`
	BeforeState   any    `json:"before_state"`
	AfterState    any    `json:"after_state"`
}

func (h *Handler) Arbitrate(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("appeals:decide") {
		return &apperr.Forbidden{Action: "arbitrate", Resource: "appeal"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var req arbitrateReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Message: "invalid request body"}
	}

	appeal, arb, err := h.service.Arbitrate(ctx, ArbitrateRequest{
		AppealID:      c.Param("id"),
		BranchID:      branchID,
		Decision:      req.Decision,
		DecisionNotes: req.DecisionNotes,
		BeforeState:   req.BeforeState,
		AfterState:    req.AfterState,
		ActorUserID:   user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"appeal":      appeal,
		"arbitration": arb,
	})
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
