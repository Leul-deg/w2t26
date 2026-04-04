// Package enrollment provides HTTP handlers for enrollment management.
package enrollment

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves enrollment routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new enrollment Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires all enrollment routes.
//
// Routes under /programs/:id/enroll are registered on the programs group.
// Routes under /enrollments/:id are registered on their own group.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	// Enroll / list enrollments by program — nested under programs.
	pg := api.Group("/programs", middlewares...)
	pg.POST("/:id/enroll", h.Enroll)
	pg.GET("/:id/enrollments", h.ListByProgram)
	pg.GET("/:id/seats", h.GetSeats)

	// Individual enrollment management.
	eg := api.Group("/enrollments", middlewares...)
	eg.GET("/:id", h.GetEnrollment)
	eg.POST("/:id/drop", h.Drop)
	eg.GET("/:id/history", h.GetHistory)

	// Reader-scoped list — /readers/:reader_id/enrollments.
	rg := api.Group("/readers", middlewares...)
	rg.GET("/:reader_id/enrollments", h.ListByReader)
}

// ── Enroll ────────────────────────────────────────────────────────────────────

type enrollReq struct {
	ReaderID          string `json:"reader_id"`
	EnrollmentChannel string `json:"enrollment_channel"`
}

func (h *Handler) Enroll(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("enrollments:write") {
		return &apperr.Forbidden{Action: "enroll", Resource: "enrollment"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var req enrollReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "", Message: "invalid request body"}
	}
	if req.ReaderID == "" {
		return &apperr.Validation{Field: "reader_id", Message: "reader_id is required"}
	}

	enrollment, err := h.service.Enroll(ctx, EnrollReaderRequest{
		ProgramID:         c.Param("id"),
		ReaderID:          req.ReaderID,
		BranchID:          branchID,
		EnrollmentChannel: req.EnrollmentChannel,
		ActorUserID:       user.User.ID,
		WorkstationID:     ctxutil.GetWorkstationID(c),
	})
	if err != nil {
		// Map EligibilityDenial to a structured 422 response so the frontend
		// can display immediate, actionable feedback.
		var ed *EligibilityDenial
		if errors.As(err, &ed) {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error":  ed.Reason,
				"detail": ed.Detail,
			})
		}
		return err
	}
	return c.JSON(http.StatusCreated, enrollment)
}

// ── Drop ──────────────────────────────────────────────────────────────────────

func (h *Handler) Drop(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("enrollments:write") {
		return &apperr.Forbidden{Action: "drop", Resource: "enrollment"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var body struct {
		ReaderID string `json:"reader_id"`
		Reason   string `json:"reason"`
	}
	if err := c.Bind(&body); err != nil {
		return &apperr.Validation{Field: "", Message: "invalid request body"}
	}
	if body.ReaderID == "" {
		return &apperr.Validation{Field: "reader_id", Message: "reader_id is required"}
	}

	if err := h.service.Drop(ctx, DropRequest{
		EnrollmentID: c.Param("id"),
		ReaderID:     body.ReaderID,
		BranchID:     branchID,
		Reason:       body.Reason,
		ActorUserID:  user.User.ID,
	}); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Get enrollment ────────────────────────────────────────────────────────────

func (h *Handler) GetEnrollment(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("enrollments:read") {
		return &apperr.Forbidden{Action: "get", Resource: "enrollment"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	e, err := h.service.GetByID(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, e)
}

// ── List by program ───────────────────────────────────────────────────────────

func (h *Handler) ListByProgram(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("enrollments:read") {
		return &apperr.Forbidden{Action: "list", Resource: "enrollments"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	result, err := h.service.ListByProgram(ctx, c.Param("id"), branchID, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ── List by reader ────────────────────────────────────────────────────────────

func (h *Handler) ListByReader(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("enrollments:read") {
		return &apperr.Forbidden{Action: "list", Resource: "reader enrollments"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	result, err := h.service.ListByReader(ctx, c.Param("reader_id"), branchID, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ── History ───────────────────────────────────────────────────────────────────

func (h *Handler) GetHistory(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("enrollments:read") {
		return &apperr.Forbidden{Action: "read", Resource: "enrollment history"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	hist, err := h.service.GetHistory(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, hist)
}

// ── Remaining seats ───────────────────────────────────────────────────────────

func (h *Handler) GetSeats(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:read") {
		return &apperr.Forbidden{Action: "read", Resource: "program seats"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	remaining, err := h.service.GetRemainingSeats(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]int{"remaining_seats": remaining})
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
