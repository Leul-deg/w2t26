package reports

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
)

// Handler serves /reports/* routes.
type Handler struct {
	service *Service
}

// NewHandler creates a new reports Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires report routes onto the given group.
func (h *Handler) RegisterRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/reports", middlewares...)

	// List available report definitions.
	g.GET("/definitions", h.ListDefinitions)

	// Run a live report query for a definition.
	g.GET("/run", h.RunReport)

	// Fetch cached aggregates for a definition and date range.
	g.GET("/aggregates", h.ListAggregates)

	// On-demand aggregate recalculation — requires reports:admin.
	g.POST("/recalculate", h.Recalculate)

	// CSV export — requires reports:export; creates an audit record.
	g.GET("/export", h.ExportReport)
}

// ListDefinitions returns all active report definitions.
func (h *Handler) ListDefinitions(c echo.Context) error {
	user := ctxutil.MustGetUser(c.Request().Context())
	if !user.HasPermission("reports:read") {
		return &apperr.Forbidden{Action: "list", Resource: "report_definitions"}
	}
	if h.service == nil {
		return echo.ErrInternalServerError
	}
	defs, err := h.service.ListDefinitions(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, defs)
}

// RunReport executes a live query for a report definition.
// Query params: definition_id, from (RFC3339), to (RFC3339), plus arbitrary filter keys.
func (h *Handler) RunReport(c echo.Context) error {
	user := ctxutil.MustGetUser(c.Request().Context())
	if !user.HasPermission("reports:read") {
		return &apperr.Forbidden{Action: "run", Resource: "report"}
	}
	if h.service == nil {
		return echo.ErrInternalServerError
	}

	branchID, _ := ctxutil.GetBranchID(c.Request().Context())
	// Administrators have branchID="" (all-branches scope). Reports run against a
	// specific branch, so admins must supply branch_id as a query parameter.
	// Non-admin callers always use their context branch; the query param is ignored.
	if branchID == "" {
		branchID = c.QueryParam("branch_id")
	}
	definitionID := c.QueryParam("definition_id")
	if definitionID == "" {
		return &apperr.Validation{Field: "definition_id", Message: "required"}
	}

	from, to, err := parseDateRange(c)
	if err != nil {
		return err
	}

	// Collect extra query params as filters (excluding reserved keys).
	filters := extractFilters(c, "definition_id", "from", "to", "branch_id")

	result, err := h.service.RunReport(c.Request().Context(), RunRequest{
		BranchID:     branchID,
		DefinitionID: definitionID,
		From:         from,
		To:           to,
		Filters:      filters,
		ActorUserID:  user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// ListAggregates returns cached aggregate records for a branch and date range.
func (h *Handler) ListAggregates(c echo.Context) error {
	user := ctxutil.MustGetUser(c.Request().Context())
	if !user.HasPermission("reports:read") {
		return &apperr.Forbidden{Action: "list", Resource: "report_aggregates"}
	}
	if h.service == nil {
		return echo.ErrInternalServerError
	}

	branchID, _ := ctxutil.GetBranchID(c.Request().Context())
	if branchID == "" {
		branchID = c.QueryParam("branch_id")
	}
	definitionID := c.QueryParam("definition_id")

	from, to, err := parseDateRange(c)
	if err != nil {
		return err
	}

	aggs, err := h.service.ListAggregates(c.Request().Context(), branchID, definitionID, from, to)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, aggs)
}

// Recalculate triggers on-demand aggregate recalculation for a date range.
// Requires reports:admin permission.
func (h *Handler) Recalculate(c echo.Context) error {
	user := ctxutil.MustGetUser(c.Request().Context())
	if !user.HasPermission("reports:admin") {
		return &apperr.Forbidden{Action: "recalculate", Resource: "report_aggregates"}
	}
	if h.service == nil {
		return echo.ErrInternalServerError
	}

	var body struct {
		DefinitionID string `json:"definition_id"`
		From         string `json:"from"`
		To           string `json:"to"`
	}
	if err := c.Bind(&body); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}

	from, err := time.Parse("2006-01-02", body.From)
	if err != nil {
		return &apperr.Validation{Field: "from", Message: "must be YYYY-MM-DD"}
	}
	to, err := time.Parse("2006-01-02", body.To)
	if err != nil {
		return &apperr.Validation{Field: "to", Message: "must be YYYY-MM-DD"}
	}

	branchID, _ := ctxutil.GetBranchID(c.Request().Context())
	count, err := h.service.RecalculateAggregates(c.Request().Context(), RecalcRequest{
		BranchID:     branchID,
		DefinitionID: body.DefinitionID,
		From:         from,
		To:           to,
		ActorUserID:  user.User.ID,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]int{"aggregates_computed": count})
}

// ExportReport generates a CSV export and returns it as a file download.
// The export is logged to export_jobs before file generation.
// Requires reports:export permission.
func (h *Handler) ExportReport(c echo.Context) error {
	user := ctxutil.MustGetUser(c.Request().Context())
	if !user.HasPermission("reports:export") {
		return &apperr.Forbidden{Action: "export", Resource: "report"}
	}
	if h.service == nil {
		return echo.ErrInternalServerError
	}

	branchID, _ := ctxutil.GetBranchID(c.Request().Context())
	if branchID == "" {
		branchID = c.QueryParam("branch_id")
	}
	definitionID := c.QueryParam("definition_id")
	if definitionID == "" {
		return &apperr.Validation{Field: "definition_id", Message: "required"}
	}

	from, to, err := parseDateRange(c)
	if err != nil {
		return err
	}

	filters := extractFilters(c, "definition_id", "from", "to", "branch_id")

	result, err := h.service.ExportReport(c.Request().Context(), ExportReportRequest{
		BranchID:      branchID,
		DefinitionID:  definitionID,
		From:          from,
		To:            to,
		Filters:       filters,
		ActorUserID:   user.User.ID,
		WorkstationID: ctxutil.GetWorkstationID(c),
	})
	if err != nil {
		return err
	}

	c.Response().Header().Set("Content-Type", "text/csv")
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+result.FileName+`"`)
	c.Response().Header().Set("X-Export-Job-ID", result.Job.ID)
	return c.Blob(http.StatusOK, "text/csv", result.Data)
}

// parseDateRange extracts and parses from/to query params (YYYY-MM-DD or RFC3339).
func parseDateRange(c echo.Context) (time.Time, time.Time, error) {
	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")

	if fromStr == "" {
		return time.Time{}, time.Time{}, &apperr.Validation{Field: "from", Message: "required"}
	}
	if toStr == "" {
		return time.Time{}, time.Time{}, &apperr.Validation{Field: "to", Message: "required"}
	}

	tryParse := func(s string) (time.Time, error) {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t, nil
		}
		return time.Parse(time.RFC3339, s)
	}

	from, err := tryParse(fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, &apperr.Validation{Field: "from", Message: "must be YYYY-MM-DD or RFC3339"}
	}
	to, err := tryParse(toStr)
	if err != nil {
		return time.Time{}, time.Time{}, &apperr.Validation{Field: "to", Message: "must be YYYY-MM-DD or RFC3339"}
	}
	return from, to, nil
}

// extractFilters returns query params not in the exclude list.
func extractFilters(c echo.Context, exclude ...string) map[string]string {
	skip := make(map[string]struct{}, len(exclude))
	for _, e := range exclude {
		skip[e] = struct{}{}
	}
	out := map[string]string{}
	for k, v := range c.QueryParams() {
		if _, ok := skip[k]; !ok && len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
