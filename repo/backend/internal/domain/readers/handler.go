// Package readers provides HTTP handlers for the reader management domain.
package readers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/audit"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler provides HTTP endpoints for reader operations.
type Handler struct {
	service     *Service
	auditLogger *audit.Logger
}

// NewHandler creates a new readers Handler.
func NewHandler(service *Service, auditLogger *audit.Logger) *Handler {
	return &Handler{
		service:     service,
		auditLogger: auditLogger,
	}
}

// RegisterRoutes registers reader routes on the given Echo group.
// Provide requireAuth and optionally branchScope as middlewares.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/readers", middlewares...)

	// Static sub-paths first to avoid :id swallowing them.
	g.GET("/statuses", h.ListStatuses)

	// Collection
	g.GET("", h.ListReaders)
	g.POST("", h.CreateReader)

	// Item
	g.GET("/:id", h.GetReader)
	g.PATCH("/:id", h.UpdateReader)
	g.PATCH("/:id/status", h.UpdateReaderStatus)
	g.GET("/:id/history", h.GetLoanHistory)
	g.GET("/:id/holdings", h.GetCurrentHoldings)
	g.POST("/:id/reveal", h.RevealSensitive)
}

// ── Response types ────────────────────────────────────────────────────────────

// readerResponse is the outbound JSON shape for a single reader.
type readerResponse struct {
	*model.Reader
	Sensitive *model.SensitiveFields `json:"sensitive_fields"`
}

func maskedResponse(r *model.Reader) readerResponse {
	return readerResponse{
		Reader:    r,
		Sensitive: model.MaskedSensitiveFields(r),
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListStatuses handles GET /api/v1/readers/statuses.
func (h *Handler) ListStatuses(c echo.Context) error {
	user, ok := ctxutil.GetUser(c.Request().Context())
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("readers:read") {
		return &apperr.Forbidden{Action: "list", Resource: "reader statuses"}
	}

	statuses, err := h.service.ListStatuses(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, statuses)
}

// ListReaders handles GET /api/v1/readers.
// Query params: search, status, page, per_page
func (h *Handler) ListReaders(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("readers:read") {
		return &apperr.Forbidden{Action: "list", Resource: "readers"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)

	var filter ReaderFilter
	if s := c.QueryParam("search"); s != "" {
		filter.Search = &s
	}
	if s := c.QueryParam("status"); s != "" {
		filter.StatusCode = &s
	}

	p := model.Pagination{
		Page:    intQueryParam(c, "page", 1),
		PerPage: intQueryParam(c, "per_page", 20),
	}

	result, err := h.service.List(ctx, branchID, filter, p)
	if err != nil {
		return err
	}

	// Mask sensitive fields in list view — they are never exposed in bulk.
	type listItem struct {
		*model.Reader
		Sensitive *model.SensitiveFields `json:"sensitive_fields"`
	}
	items := make([]listItem, len(result.Items))
	for i, r := range result.Items {
		items[i] = listItem{
			Reader:    r,
			Sensitive: model.MaskedSensitiveFields(r),
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"items":       items,
		"total":       result.Total,
		"page":        result.Page,
		"per_page":    result.PerPage,
		"total_pages": result.TotalPages,
	})
}

// createReaderRequest is the JSON body for POST /api/v1/readers.
type createReaderRequest struct {
	BranchID      string  `json:"branch_id"`
	ReaderNumber  string  `json:"reader_number"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	PreferredName *string `json:"preferred_name"`
	Notes         *string `json:"notes"`
	NationalID    *string `json:"national_id"`
	ContactEmail  *string `json:"contact_email"`
	ContactPhone  *string `json:"contact_phone"`
	DateOfBirth   *string `json:"date_of_birth"`
}

// CreateReader handles POST /api/v1/readers.
func (h *Handler) CreateReader(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("readers:write") {
		return &apperr.Forbidden{Action: "create", Resource: "reader"}
	}

	var req createReaderRequest
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}

	// If no branch_id given in body, use the caller's effective branch scope.
	if req.BranchID == "" {
		req.BranchID, _ = ctxutil.GetBranchID(ctx)
	}

	reader, err := h.service.Create(ctx, CreateRequest{
		BranchID:      req.BranchID,
		ReaderNumber:  req.ReaderNumber,
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		PreferredName: req.PreferredName,
		Notes:         req.Notes,
		NationalID:    req.NationalID,
		ContactEmail:  req.ContactEmail,
		ContactPhone:  req.ContactPhone,
		DateOfBirth:   req.DateOfBirth,
		ActorUserID:   user.User.ID,
	})
	if err != nil {
		return err
	}

	h.auditLogger.LogAdminChange(ctx,
		user.User.ID, user.User.Username,
		model.AuditReaderCreated, "reader", reader.ID,
		ctxutil.GetWorkstationID(c),
		nil, map[string]string{"reader_number": reader.ReaderNumber},
	)

	return c.JSON(http.StatusCreated, maskedResponse(reader))
}

// GetReader handles GET /api/v1/readers/:id.
// Returns the reader with masked sensitive fields.
func (h *Handler) GetReader(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("readers:read") {
		return &apperr.Forbidden{Action: "read", Resource: "reader"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	reader, err := h.service.GetByID(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, maskedResponse(reader))
}

// updateReaderRequest is the JSON body for PATCH /api/v1/readers/:id.
type updateReaderRequest struct {
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	PreferredName *string `json:"preferred_name"`
	Notes         *string `json:"notes"`
	NationalID    *string `json:"national_id"`
	ContactEmail  *string `json:"contact_email"`
	ContactPhone  *string `json:"contact_phone"`
	DateOfBirth   *string `json:"date_of_birth"`
}

// UpdateReader handles PATCH /api/v1/readers/:id.
func (h *Handler) UpdateReader(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("readers:write") {
		return &apperr.Forbidden{Action: "update", Resource: "reader"}
	}

	var req updateReaderRequest
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	reader, err := h.service.Update(ctx, c.Param("id"), branchID, UpdateRequest{
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		PreferredName: req.PreferredName,
		Notes:         req.Notes,
		NationalID:    req.NationalID,
		ContactEmail:  req.ContactEmail,
		ContactPhone:  req.ContactPhone,
		DateOfBirth:   req.DateOfBirth,
	})
	if err != nil {
		return err
	}

	h.auditLogger.LogAdminChange(ctx,
		user.User.ID, user.User.Username,
		model.AuditReaderUpdated, "reader", reader.ID,
		ctxutil.GetWorkstationID(c),
		nil, map[string]string{"action": "updated"},
	)

	return c.JSON(http.StatusOK, maskedResponse(reader))
}

// updateStatusRequest is the JSON body for PATCH /api/v1/readers/:id/status.
type updateStatusRequest struct {
	StatusCode string `json:"status_code"`
}

// UpdateReaderStatus handles PATCH /api/v1/readers/:id/status.
func (h *Handler) UpdateReaderStatus(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("readers:write") {
		return &apperr.Forbidden{Action: "update status", Resource: "reader"}
	}

	var req updateStatusRequest
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.StatusCode == "" {
		return &apperr.Validation{Field: "status_code", Message: "status_code is required"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	readerID := c.Param("id")

	if err := h.service.UpdateStatus(ctx, readerID, branchID, user.User.ID, req.StatusCode); err != nil {
		return err
	}

	h.auditLogger.LogAdminChange(ctx,
		user.User.ID, user.User.Username,
		model.AuditReaderStatusChanged, "reader", readerID,
		ctxutil.GetWorkstationID(c),
		nil, map[string]string{"new_status": req.StatusCode},
	)

	return c.JSON(http.StatusOK, map[string]string{"status_code": req.StatusCode})
}

// GetLoanHistory handles GET /api/v1/readers/:id/history.
func (h *Handler) GetLoanHistory(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("readers:read") {
		return &apperr.Forbidden{Action: "read", Resource: "reader loan history"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	p := model.Pagination{
		Page:    intQueryParam(c, "page", 1),
		PerPage: intQueryParam(c, "per_page", 20),
	}

	result, err := h.service.GetLoanHistory(ctx, c.Param("id"), branchID, p)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// GetCurrentHoldings handles GET /api/v1/readers/:id/holdings.
func (h *Handler) GetCurrentHoldings(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("readers:read") {
		return &apperr.Forbidden{Action: "read", Resource: "reader current holdings"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	items, err := h.service.GetCurrentHoldings(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	if items == nil {
		items = []*LoanHistoryItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// stepUpWindowMinutes is how long a completed step-up grants access to sensitive fields.
const stepUpWindowMinutes = 15

// RevealSensitive handles POST /api/v1/readers/:id/reveal.
// Requires "readers:reveal_sensitive" permission AND a step-up completed within
// the last 15 minutes (POST /auth/stepup sets session.stepup_at server-side).
// An audit event is always logged regardless of whether decryption succeeds.
func (h *Handler) RevealSensitive(c echo.Context) error {
	ctx := c.Request().Context()
	readerID := c.Param("id")

	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("readers:reveal_sensitive") {
		return &apperr.Forbidden{Action: "reveal", Resource: "reader sensitive fields"}
	}

	// Server-side step-up enforcement: the session must have a recent stepup_at.
	session, _ := ctxutil.GetSession(ctx)
	if !session.HasRecentStepUp(stepUpWindowMinutes) {
		return &apperr.Forbidden{Action: "reveal", Resource: "reader sensitive fields — step-up required"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)

	// Log the reveal attempt before decryption — audit must capture the intent
	// regardless of whether decryption succeeds.
	h.auditLogger.LogSensitiveReveal(ctx, user.User.ID, user.User.Username, readerID)

	sf, err := h.service.RevealSensitive(ctx, readerID, branchID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, sf)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// intQueryParam reads an integer query parameter, returning defaultVal on absence or error.
func intQueryParam(c echo.Context, name string, defaultVal int) int {
	s := c.QueryParam(name)
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}
