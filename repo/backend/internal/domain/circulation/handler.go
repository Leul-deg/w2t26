// Package circulation provides HTTP handlers for the circulation domain.
package circulation

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves /circulation/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new circulation Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all circulation routes.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/circulation", middlewares...)
	g.POST("/checkout", h.Checkout)
	g.POST("/return", h.Return)
	g.GET("", h.ListByBranch)
	g.GET("/copy/:id", h.ListByCopy)
	g.GET("/reader/:id", h.ListByReader)
	g.GET("/active/:id", h.GetActiveCheckout)
}

type checkoutReq struct {
	CopyID        string `json:"copy_id"`
	Barcode       string `json:"barcode"`
	ReaderID      string `json:"reader_id"`
	// BranchID is used by administrators (context branchID = "") to specify the
	// operating branch. Non-admin callers always use their context branch.
	BranchID      string `json:"branch_id"`
	DueDate       string `json:"due_date"`
	WorkstationID string `json:"workstation_id"`
	Notes         string `json:"notes"`
}

// Checkout handles POST /circulation/checkout.
func (h *Handler) Checkout(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("circulation:write") {
		return &apperr.Forbidden{Action: "checkout", Resource: "copy"}
	}

	var req checkoutReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	// Administrators have branchID="" (all-branches scope). For checkout, which must
	// record a specific branch, they supply branch_id in the request body.
	if branchID == "" {
		branchID = req.BranchID
	}
	event, err := h.service.Checkout(ctx, CheckoutRequest{
		CopyID:        req.CopyID,
		Barcode:       req.Barcode,
		ReaderID:      req.ReaderID,
		BranchID:      branchID,
		DueDate:       req.DueDate,
		WorkstationID: req.WorkstationID,
		Notes:         req.Notes,
		ActorUserID:   user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, event)
}

type returnReq struct {
	CopyID        string `json:"copy_id"`
	Barcode       string `json:"barcode"`
	// BranchID is used by administrators (context branchID = "") to specify the
	// operating branch. Non-admin callers always use their context branch.
	BranchID      string `json:"branch_id"`
	WorkstationID string `json:"workstation_id"`
	Notes         string `json:"notes"`
}

// Return handles POST /circulation/return.
func (h *Handler) Return(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("circulation:write") {
		return &apperr.Forbidden{Action: "return", Resource: "copy"}
	}

	var req returnReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	if branchID == "" {
		branchID = req.BranchID
	}
	event, err := h.service.Return(ctx, ReturnRequest{
		CopyID:        req.CopyID,
		Barcode:       req.Barcode,
		BranchID:      branchID,
		WorkstationID: req.WorkstationID,
		Notes:         req.Notes,
		ActorUserID:   user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, event)
}

// ListByBranch handles GET /circulation.
func (h *Handler) ListByBranch(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("circulation:read") {
		return &apperr.Forbidden{Action: "list", Resource: "circulation events"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	var f CirculationFilter
	if v := c.QueryParam("event_type"); v != "" {
		f.EventType = &v
	}
	if v := c.QueryParam("reader_id"); v != "" {
		f.ReaderID = &v
	}
	if v := c.QueryParam("copy_id"); v != "" {
		f.CopyID = &v
	}

	result, err := h.service.ListByBranch(ctx, branchID, f, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ListByCopy handles GET /circulation/copy/:id.
func (h *Handler) ListByCopy(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("circulation:read") {
		return &apperr.Forbidden{Action: "list", Resource: "copy circulation events"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	result, err := h.service.ListByCopy(ctx, c.Param("id"), branchID, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ListByReader handles GET /circulation/reader/:id.
func (h *Handler) ListByReader(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("circulation:read") {
		return &apperr.Forbidden{Action: "list", Resource: "reader circulation events"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	result, err := h.service.ListByReader(ctx, c.Param("id"), branchID, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// GetActiveCheckout handles GET /circulation/active/:id.
func (h *Handler) GetActiveCheckout(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("circulation:read") {
		return &apperr.Forbidden{Action: "read", Resource: "active checkout"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	event, err := h.service.GetActiveCheckout(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, event)
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
