// Package programs provides HTTP handlers for program management.
package programs

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

// Handler serves /programs/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new programs Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires all program routes.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/programs", middlewares...)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Update)
	g.PATCH("/:id/status", h.UpdateStatus)

	// Prerequisites
	g.GET("/:id/prerequisites", h.ListPrerequisites)
	g.POST("/:id/prerequisites", h.AddPrerequisite)
	g.DELETE("/:id/prerequisites/:req_id", h.RemovePrerequisite)

	// Enrollment rules
	g.GET("/:id/rules", h.ListRules)
	g.POST("/:id/rules", h.AddRule)
	g.DELETE("/:id/rules/:rule_id", h.RemoveRule)
}

// ── List ──────────────────────────────────────────────────────────────────────

func (h *Handler) List(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:read") {
		return &apperr.Forbidden{Action: "list", Resource: "programs"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	f := ProgramFilter{}
	if s := c.QueryParam("status"); s != "" {
		f.Status = &s
	}
	if s := c.QueryParam("category"); s != "" {
		f.Category = &s
	}
	if s := c.QueryParam("search"); s != "" {
		f.Search = &s
	}
	result, err := h.service.List(ctx, branchID, f, parsePagination(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ── Create ────────────────────────────────────────────────────────────────────

type createProgramReq struct {
	// BranchID is used by administrators (whose context branchID is "") to specify
	// which branch to create the program in. Non-admin callers always use their
	// context branch; any BranchID in the body is ignored for them.
	BranchID           string     `json:"branch_id"`
	Title              string     `json:"title"`
	Description        *string    `json:"description"`
	Category           *string    `json:"category"`
	VenueType          *string    `json:"venue_type"`
	VenueName          *string    `json:"venue_name"`
	Capacity           int        `json:"capacity"`
	EnrollmentOpensAt  *time.Time `json:"enrollment_opens_at"`
	EnrollmentClosesAt *time.Time `json:"enrollment_closes_at"`
	StartsAt           time.Time  `json:"starts_at"`
	EndsAt             time.Time  `json:"ends_at"`
	EnrollmentChannel  string     `json:"enrollment_channel"`
}

func (h *Handler) Create(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:write") {
		return &apperr.Forbidden{Action: "create", Resource: "program"}
	}
	var req createProgramReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "", Message: "invalid request body"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	// Administrators have an empty context branchID ("" = all-branches scope).
	// They must supply a branch_id in the request body to target a specific branch.
	// Non-admin callers always use their context branch; body branch_id is ignored.
	if branchID == "" {
		branchID = req.BranchID
	}

	p, err := h.service.Create(ctx, CreateRequest{
		BranchID:           branchID,
		Title:              req.Title,
		Description:        req.Description,
		Category:           req.Category,
		VenueType:          req.VenueType,
		VenueName:          req.VenueName,
		Capacity:           req.Capacity,
		EnrollmentOpensAt:  req.EnrollmentOpensAt,
		EnrollmentClosesAt: req.EnrollmentClosesAt,
		StartsAt:           req.StartsAt,
		EndsAt:             req.EndsAt,
		EnrollmentChannel:  req.EnrollmentChannel,
		ActorUserID:        user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, p)
}

// ── Get ───────────────────────────────────────────────────────────────────────

type programDetailResponse struct {
	*model.Program
	Prerequisites  []*model.ProgramPrerequisite `json:"prerequisites"`
	Rules          []*model.EnrollmentRule       `json:"enrollment_rules"`
}

func (h *Handler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:read") {
		return &apperr.Forbidden{Action: "get", Resource: "program"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	p, err := h.service.Get(ctx, c.Param("id"), branchID)
	if err != nil {
		return err
	}
	prereqs, err := h.service.GetPrerequisites(ctx, p.ID)
	if err != nil {
		return err
	}
	rules, err := h.service.GetEnrollmentRules(ctx, p.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, programDetailResponse{Program: p, Prerequisites: prereqs, Rules: rules})
}

// ── Update ────────────────────────────────────────────────────────────────────

type updateProgramReq struct {
	Title              string     `json:"title"`
	Description        *string    `json:"description"`
	Category           *string    `json:"category"`
	VenueType          *string    `json:"venue_type"`
	VenueName          *string    `json:"venue_name"`
	Capacity           int        `json:"capacity"`
	EnrollmentOpensAt  *time.Time `json:"enrollment_opens_at"`
	EnrollmentClosesAt *time.Time `json:"enrollment_closes_at"`
	StartsAt           time.Time  `json:"starts_at"`
	EndsAt             time.Time  `json:"ends_at"`
	EnrollmentChannel  string     `json:"enrollment_channel"`
}

func (h *Handler) Update(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:write") {
		return &apperr.Forbidden{Action: "update", Resource: "program"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var req updateProgramReq
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "", Message: "invalid request body"}
	}

	p, err := h.service.Update(ctx, c.Param("id"), branchID, user.User.ID, UpdateRequest{
		Title:              req.Title,
		Description:        req.Description,
		Category:           req.Category,
		VenueType:          req.VenueType,
		VenueName:          req.VenueName,
		Capacity:           req.Capacity,
		EnrollmentOpensAt:  req.EnrollmentOpensAt,
		EnrollmentClosesAt: req.EnrollmentClosesAt,
		StartsAt:           req.StartsAt,
		EndsAt:             req.EndsAt,
		EnrollmentChannel:  req.EnrollmentChannel,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, p)
}

// ── Update status ─────────────────────────────────────────────────────────────

func (h *Handler) UpdateStatus(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:write") {
		return &apperr.Forbidden{Action: "update_status", Resource: "program"}
	}
	branchID, _ := ctxutil.GetBranchID(ctx)

	var body struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&body); err != nil {
		return &apperr.Validation{Field: "status", Message: "invalid request body"}
	}
	if err := h.service.UpdateStatus(ctx, c.Param("id"), branchID, user.User.ID, body.Status); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"status": body.Status})
}

// ── Prerequisites ─────────────────────────────────────────────────────────────

func (h *Handler) ListPrerequisites(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:read") {
		return &apperr.Forbidden{Action: "list", Resource: "prerequisites"}
	}
	// Verify the parent program is accessible to the caller's branch before
	// returning its prerequisites — prevents cross-branch data leakage via ID.
	branchID, _ := ctxutil.GetBranchID(ctx)
	if _, err := h.service.Get(ctx, c.Param("id"), branchID); err != nil {
		return err
	}
	prereqs, err := h.service.GetPrerequisites(ctx, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, prereqs)
}

func (h *Handler) AddPrerequisite(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:write") {
		return &apperr.Forbidden{Action: "add", Resource: "prerequisite"}
	}

	var body struct {
		RequiredProgramID string  `json:"required_program_id"`
		Description       *string `json:"description"`
	}
	if err := c.Bind(&body); err != nil {
		return &apperr.Validation{Field: "", Message: "invalid request body"}
	}

	// Verify the parent program is accessible before mutating its prerequisites.
	branchID, _ := ctxutil.GetBranchID(ctx)
	if _, err := h.service.Get(ctx, c.Param("id"), branchID); err != nil {
		return err
	}

	pr, err := h.service.AddPrerequisite(ctx, c.Param("id"), body.RequiredProgramID, body.Description)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, pr)
}

func (h *Handler) RemovePrerequisite(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:write") {
		return &apperr.Forbidden{Action: "remove", Resource: "prerequisite"}
	}
	// Verify the parent program is accessible before mutating its prerequisites.
	branchID, _ := ctxutil.GetBranchID(ctx)
	if _, err := h.service.Get(ctx, c.Param("id"), branchID); err != nil {
		return err
	}
	return h.service.RemovePrerequisite(ctx, c.Param("id"), c.Param("req_id"))
}

// ── Enrollment rules ──────────────────────────────────────────────────────────

func (h *Handler) ListRules(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:read") {
		return &apperr.Forbidden{Action: "list", Resource: "enrollment rules"}
	}
	// Verify the parent program is accessible to the caller's branch before
	// returning its rules — prevents cross-branch data leakage via ID guessing.
	branchID, _ := ctxutil.GetBranchID(ctx)
	if _, err := h.service.Get(ctx, c.Param("id"), branchID); err != nil {
		return err
	}
	rules, err := h.service.GetEnrollmentRules(ctx, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, rules)
}

func (h *Handler) AddRule(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:write") {
		return &apperr.Forbidden{Action: "add", Resource: "enrollment rule"}
	}

	var body struct {
		RuleType   string  `json:"rule_type"`
		MatchField string  `json:"match_field"`
		MatchValue string  `json:"match_value"`
		Reason     *string `json:"reason"`
	}
	if err := c.Bind(&body); err != nil {
		return &apperr.Validation{Field: "", Message: "invalid request body"}
	}

	// Verify the parent program is accessible before mutating its rules.
	branchID, _ := ctxutil.GetBranchID(ctx)
	if _, err := h.service.Get(ctx, c.Param("id"), branchID); err != nil {
		return err
	}

	rule, err := h.service.AddEnrollmentRule(ctx, c.Param("id"), body.RuleType, body.MatchField, body.MatchValue, body.Reason)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, rule)
}

func (h *Handler) RemoveRule(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("programs:write") {
		return &apperr.Forbidden{Action: "remove", Resource: "enrollment rule"}
	}
	// Verify the parent program is accessible before mutating its rules.
	branchID, _ := ctxutil.GetBranchID(ctx)
	if _, err := h.service.Get(ctx, c.Param("id"), branchID); err != nil {
		return err
	}
	// Pass programID alongside ruleID so the repo can scope the DELETE to
	// this program, preventing cross-program IDOR via a known rule UUID.
	return h.service.RemoveEnrollmentRule(ctx, c.Param("id"), c.Param("rule_id"))
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
