// Package holdings provides HTTP handlers for the holdings and copies domain.
package holdings

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves both /holdings/* and /copies/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new holdings Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all holdings and copies routes.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	hg := api.Group("/holdings", middlewares...)
	hg.GET("", h.ListHoldings)
	hg.POST("", h.CreateHolding)
	hg.GET("/:id", h.GetHolding)
	hg.PATCH("/:id", h.UpdateHolding)
	hg.DELETE("/:id", h.DeactivateHolding)
	hg.GET("/:id/copies", h.ListCopies)
	hg.POST("/:id/copies", h.AddCopy)

	cg := api.Group("/copies", middlewares...)
	cg.GET("/statuses", h.ListCopyStatuses)
	cg.GET("/lookup", h.LookupCopyByBarcode)
	cg.GET("/:id", h.GetCopy)
	cg.PATCH("/:id", h.UpdateCopy)
	cg.PATCH("/:id/status", h.UpdateCopyStatus)
}

// ── Holdings ──────────────────────────────────────────────────────────────────

type createHoldingReq struct {
	BranchID        string  `json:"branch_id"`
	Title           string  `json:"title"`
	Author          *string `json:"author"`
	ISBN            *string `json:"isbn"`
	Publisher       *string `json:"publisher"`
	PublicationYear *int    `json:"publication_year"`
	Category        *string `json:"category"`
	Subcategory     *string `json:"subcategory"`
	Language        string  `json:"language"`
	Description     *string `json:"description"`
}

func (h *Handler) ListHoldings(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("holdings:read") {
		return &apperr.Forbidden{Action: "list", Resource: "holdings"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var f HoldingFilter
	if s := c.QueryParam("search"); s != "" {
		f.Search = &s
	}
	if s := c.QueryParam("category"); s != "" {
		f.Category = &s
	}
	if s := c.QueryParam("isbn"); s != "" {
		f.ISBN = &s
	}
	if s := c.QueryParam("active"); s != "" {
		b := s == "true" || s == "1"
		f.Active = &b
	}

	p := parsePagination(c)
	result, err := h.service.ListHoldings(ctx, branchID, f, p)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) CreateHolding(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("holdings:write") {
		return &apperr.Forbidden{Action: "create", Resource: "holding"}
	}

	var req createHoldingReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.BranchID == "" {
		req.BranchID, _ = ctxutil.GetBranchID(ctx)
	}

	holding, err := h.service.CreateHolding(ctx, CreateHoldingRequest{
		BranchID:        req.BranchID,
		Title:           req.Title,
		Author:          req.Author,
		ISBN:            req.ISBN,
		Publisher:       req.Publisher,
		PublicationYear: req.PublicationYear,
		Category:        req.Category,
		Subcategory:     req.Subcategory,
		Language:        req.Language,
		Description:     req.Description,
		ActorUserID:     user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, holding)
}

func (h *Handler) GetHolding(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("holdings:read") {
		return &apperr.Forbidden{Action: "read", Resource: "holding"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	holding, err := h.service.GetHolding(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, holding)
}

type updateHoldingReq struct {
	Title           string  `json:"title"`
	Author          *string `json:"author"`
	ISBN            *string `json:"isbn"`
	Publisher       *string `json:"publisher"`
	PublicationYear *int    `json:"publication_year"`
	Category        *string `json:"category"`
	Subcategory     *string `json:"subcategory"`
	Language        string  `json:"language"`
	Description     *string `json:"description"`
}

func (h *Handler) UpdateHolding(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("holdings:write") {
		return &apperr.Forbidden{Action: "update", Resource: "holding"}
	}

	var req updateHoldingReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	holding, err := h.service.UpdateHolding(ctx, c.Param("id"), branchID, user.User.ID, UpdateHoldingRequest{
		Title: req.Title, Author: req.Author, ISBN: req.ISBN,
		Publisher: req.Publisher, PublicationYear: req.PublicationYear,
		Category: req.Category, Subcategory: req.Subcategory,
		Language: req.Language, Description: req.Description,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, holding)
}

func (h *Handler) DeactivateHolding(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("holdings:write") {
		return &apperr.Forbidden{Action: "deactivate", Resource: "holding"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	if err := h.service.DeactivateHolding(ctx, c.Param("id"), branchID, user.User.ID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Copies ────────────────────────────────────────────────────────────────────

func (h *Handler) ListCopyStatuses(c echo.Context) error {
	ctx := c.Request().Context()
	if _, ok := ctxutil.GetUser(ctx); !ok {
		return &apperr.Unauthorized{}
	}
	statuses, err := h.service.ListCopyStatuses(ctx)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, statuses)
}

// LookupCopyByBarcode handles GET /copies/lookup?barcode=...
func (h *Handler) LookupCopyByBarcode(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("copies:read") {
		return &apperr.Forbidden{Action: "lookup", Resource: "copy"}
	}

	barcode := c.QueryParam("barcode")
	if barcode == "" {
		return &apperr.Validation{Field: "barcode", Message: "barcode query param is required"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	copy, err := h.service.GetCopyByBarcode(ctx, barcode, branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, copy)
}

func (h *Handler) ListCopies(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("copies:read") {
		return &apperr.Forbidden{Action: "list", Resource: "copies"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	p := parsePagination(c)
	result, err := h.service.ListCopies(ctx, c.Param("id"), branchID, p)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

type addCopyReq struct {
	Barcode       string   `json:"barcode"`
	StatusCode    string   `json:"status_code"`
	Condition     string   `json:"condition"`
	ShelfLocation *string  `json:"shelf_location"`
	AcquiredAt    *string  `json:"acquired_at"`
	PricePaid     *float64 `json:"price_paid"`
	Notes         *string  `json:"notes"`
}

func (h *Handler) AddCopy(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("copies:write") {
		return &apperr.Forbidden{Action: "add copy", Resource: "holding"}
	}

	var req addCopyReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	copy, err := h.service.AddCopy(ctx, c.Param("id"), branchID, user.User.ID, AddCopyRequest{
		Barcode:       req.Barcode,
		StatusCode:    req.StatusCode,
		Condition:     req.Condition,
		ShelfLocation: req.ShelfLocation,
		AcquiredAt:    req.AcquiredAt,
		PricePaid:     req.PricePaid,
		Notes:         req.Notes,
		ActorUserID:   user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, copy)
}

func (h *Handler) GetCopy(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("copies:read") {
		return &apperr.Forbidden{Action: "read", Resource: "copy"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)
	copy, err := h.service.GetCopyByID(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, copy)
}

type updateCopyReq struct {
	Condition     string   `json:"condition"`
	ShelfLocation *string  `json:"shelf_location"`
	AcquiredAt    *string  `json:"acquired_at"`
	PricePaid     *float64 `json:"price_paid"`
	Notes         *string  `json:"notes"`
}

func (h *Handler) UpdateCopy(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("copies:write") {
		return &apperr.Forbidden{Action: "update", Resource: "copy"}
	}

	var req updateCopyReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	copy, err := h.service.UpdateCopy(ctx, c.Param("id"), branchID, user.User.ID, UpdateCopyRequest{
		Condition:     req.Condition,
		ShelfLocation: req.ShelfLocation,
		AcquiredAt:    req.AcquiredAt,
		PricePaid:     req.PricePaid,
		Notes:         req.Notes,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, copy)
}

type updateCopyStatusReq struct {
	StatusCode string `json:"status_code"`
}

func (h *Handler) UpdateCopyStatus(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("holdings:write") {
		return &apperr.Forbidden{Action: "update status", Resource: "copy"}
	}

	var req updateCopyStatusReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.StatusCode == "" {
		return &apperr.Validation{Field: "status_code", Message: "status_code is required"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	if err := h.service.UpdateCopyStatus(ctx, c.Param("id"), branchID, user.User.ID, req.StatusCode); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"status_code": req.StatusCode})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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
