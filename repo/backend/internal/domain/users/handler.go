package users

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"lms/internal/apperr"
	"lms/internal/ctxutil"
	"lms/internal/model"
)

const (
	sessionCookieName = "lms_session"
	sessionCookiePath = "/"
)

// Handler holds the HTTP handlers for authentication and user management routes.
type Handler struct {
	service  *Service
	userRepo Repository
}

// NewHandler creates a new auth Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// NewHandlerWithRepo creates a Handler that also handles user management routes.
func NewHandlerWithRepo(service *Service, userRepo Repository) *Handler {
	return &Handler{service: service, userRepo: userRepo}
}

// RegisterRoutes registers authentication routes on the given Echo group.
// The group is expected to be the /api/v1 group.
func (h *Handler) RegisterRoutes(api *echo.Group, authMW ...echo.MiddlewareFunc) {
	auth := api.Group("/auth")
	auth.POST("/login", h.Login)

	// Protected routes require an active session.
	if len(authMW) > 0 {
		auth.GET("/me", h.Me, authMW...)
		auth.POST("/logout", h.Logout, authMW...)
		auth.POST("/stepup", h.StepUp, authMW...)
	} else {
		auth.GET("/me", h.Me)
		auth.POST("/logout", h.Logout)
		auth.POST("/stepup", h.StepUp)
	}
}

// loginRequest is the JSON body for POST /api/v1/auth/login.
type loginRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	CaptchaKey    string `json:"captcha_key"`
	CaptchaAnswer string `json:"captcha_answer"`
}

// loginResponse is the JSON body returned on successful login.
type loginResponse struct {
	User            *model.UserWithRoles `json:"user"`
	CaptchaRequired bool                 `json:"captcha_required"`
}

// Login handles POST /api/v1/auth/login.
// On success, sets an httpOnly session cookie and returns 200 with the user object.
// Sensitive data (password_hash) is never serialised via model tags (json:"-").
func (h *Handler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.Username == "" || req.Password == "" {
		return &apperr.Validation{Field: "username", Message: "username and password are required"}
	}

	workstationID := ctxutil.GetWorkstationID(c)
	ipAddr := ctxutil.GetIPAddress(c)

	result, err := h.service.Login(
		c.Request().Context(),
		req.Username, req.Password,
		req.CaptchaKey, req.CaptchaAnswer,
		workstationID, ipAddr,
	)
	if err != nil {
		return err
	}

	// Set the session cookie — httpOnly, SameSite=Strict, no Secure flag
	// (offline-first deployment with no TLS assumed at this stage).
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    result.SessionToken,
		Path:     sessionCookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
		MaxAge:   0, // session cookie — expires when browser closes
		Expires:  result.Session.ExpiresAt,
	}
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, loginResponse{
		User:            result.User,
		CaptchaRequired: false,
	})
}

// Logout handles POST /api/v1/auth/logout.
// Invalidates the session and clears the cookie. Returns 204 No Content.
func (h *Handler) Logout(c echo.Context) error {
	ctx := c.Request().Context()
	session, ok := ctxutil.GetSession(ctx)
	if !ok || session == nil {
		return &apperr.Unauthorized{}
	}
	user, _ := ctxutil.GetUser(ctx)

	var userID, username string
	if user != nil && user.User != nil {
		userID = user.User.ID
		username = user.User.Username
	}

	workstationID := ctxutil.GetWorkstationID(c)
	if err := h.service.Logout(ctx, session.ID, userID, username, workstationID); err != nil {
		return err
	}

	// Clear the session cookie by expiring it immediately.
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     sessionCookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	return c.NoContent(http.StatusNoContent)
}

// RegisterUserRoutes registers user management routes on the /api/v1/users group.
// Requires NewHandlerWithRepo so that userRepo is available.
func (h *Handler) RegisterUserRoutes(api *echo.Group, middlewares ...echo.MiddlewareFunc) {
	g := api.Group("/users", middlewares...)

	g.GET("", h.ListUsers)
	g.POST("", h.CreateUser)
	g.GET("/:id", h.GetUser)
	g.PATCH("/:id", h.UpdateUser)

	// Role assignment
	g.POST("/:id/roles", h.AssignRole)
	g.DELETE("/:id/roles/:role_id", h.RevokeRole)

	// Branch assignment
	g.POST("/:id/branches", h.AssignBranch)
	g.DELETE("/:id/branches/:branch_id", h.RevokeBranch)
}

// ── User management handlers ──────────────────────────────────────────────────

// ListUsers handles GET /api/v1/users.
func (h *Handler) ListUsers(c echo.Context) error {
	ctx := c.Request().Context()
	user, ok := ctxutil.GetUser(ctx)
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	if !user.HasPermission("users:read") {
		return &apperr.Forbidden{Action: "list", Resource: "users"}
	}

	branchID, _ := ctxutil.GetBranchID(ctx)
	// branchID="" means administrator (all branches).
	// branchID="00000000-..." is the sentinel for unassigned non-admins — it must
	// NOT be coerced to "" here because userRepo.List("") returns all users.
	// Passing the sentinel UUID as-is yields WHERE uba.branch_id = '00000000-...'
	// which matches no real branch assignment → empty result set, which is correct.

	p := model.Pagination{
		Page:    userIntQueryParam(c, "page", 1),
		PerPage: userIntQueryParam(c, "per_page", 20),
	}

	result, err := h.userRepo.List(ctx, branchID, p)
	if err != nil {
		return err
	}

	type userItem struct {
		*model.User
		Roles    []*model.Role `json:"roles"`
		Branches []string      `json:"branch_ids"`
	}

	items := make([]userItem, len(result.Items))
	for i, u := range result.Items {
		roles, _ := h.userRepo.GetRoles(ctx, u.ID)
		branches, _ := h.userRepo.GetBranches(ctx, u.ID)
		if roles == nil {
			roles = []*model.Role{}
		}
		if branches == nil {
			branches = []string{}
		}
		items[i] = userItem{User: u, Roles: roles, Branches: branches}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"items":       items,
		"total":       result.Total,
		"page":        result.Page,
		"per_page":    result.PerPage,
		"total_pages": result.TotalPages,
	})
}

// createUserRequest is the JSON body for POST /api/v1/users.
type createUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	IsActive *bool  `json:"is_active"`
}

// CreateUser handles POST /api/v1/users.
func (h *Handler) CreateUser(c echo.Context) error {
	ctx := c.Request().Context()
	caller, ok := ctxutil.GetUser(ctx)
	if !ok || caller == nil {
		return &apperr.Unauthorized{}
	}
	if !caller.HasPermission("users:write") {
		return &apperr.Forbidden{Action: "create", Resource: "user"}
	}

	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return &apperr.Validation{Field: "username", Message: "username, email, and password are required"}
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		return &apperr.Internal{Cause: err}
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	u := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		IsActive:     isActive,
	}
	if err := h.userRepo.Create(ctx, u); err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, u)
}

// GetUser handles GET /api/v1/users/:id.
func (h *Handler) GetUser(c echo.Context) error {
	ctx := c.Request().Context()
	caller, ok := ctxutil.GetUser(ctx)
	if !ok || caller == nil {
		return &apperr.Unauthorized{}
	}
	if !caller.HasPermission("users:read") {
		return &apperr.Forbidden{Action: "read", Resource: "user"}
	}

	u, err := h.userRepo.GetByID(ctx, c.Param("id"))
	if err != nil {
		return err
	}

	roles, _ := h.userRepo.GetRoles(ctx, u.ID)
	branches, _ := h.userRepo.GetBranches(ctx, u.ID)
	if roles == nil {
		roles = []*model.Role{}
	}
	if branches == nil {
		branches = []string{}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"user":       u,
		"roles":      roles,
		"branch_ids": branches,
	})
}

// updateUserRequest is the JSON body for PATCH /api/v1/users/:id.
type updateUserRequest struct {
	Email    *string `json:"email"`
	IsActive *bool   `json:"is_active"`
}

// UpdateUser handles PATCH /api/v1/users/:id.
// Allows updating email and active status.
func (h *Handler) UpdateUser(c echo.Context) error {
	ctx := c.Request().Context()
	caller, ok := ctxutil.GetUser(ctx)
	if !ok || caller == nil {
		return &apperr.Unauthorized{}
	}
	if !caller.HasPermission("users:write") {
		return &apperr.Forbidden{Action: "update", Resource: "user"}
	}

	var req updateUserRequest
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}

	u, err := h.userRepo.GetByID(ctx, c.Param("id"))
	if err != nil {
		return err
	}

	if req.Email != nil {
		u.Email = *req.Email
	}
	if req.IsActive != nil {
		u.IsActive = *req.IsActive
	}

	if err := h.userRepo.Update(ctx, u); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, u)
}

// assignRoleRequest is the JSON body for POST /api/v1/users/:id/roles.
type assignRoleRequest struct {
	RoleID string `json:"role_id"`
}

// AssignRole handles POST /api/v1/users/:id/roles.
func (h *Handler) AssignRole(c echo.Context) error {
	ctx := c.Request().Context()
	caller, ok := ctxutil.GetUser(ctx)
	if !ok || caller == nil {
		return &apperr.Unauthorized{}
	}
	if !caller.HasPermission("users:admin") {
		return &apperr.Forbidden{Action: "assign role", Resource: "user"}
	}

	var req assignRoleRequest
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.RoleID == "" {
		return &apperr.Validation{Field: "role_id", Message: "role_id is required"}
	}

	if err := h.userRepo.AssignRole(ctx, c.Param("id"), req.RoleID, caller.User.ID); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{"role_id": req.RoleID})
}

// RevokeRole handles DELETE /api/v1/users/:id/roles/:role_id.
func (h *Handler) RevokeRole(c echo.Context) error {
	ctx := c.Request().Context()
	caller, ok := ctxutil.GetUser(ctx)
	if !ok || caller == nil {
		return &apperr.Unauthorized{}
	}
	if !caller.HasPermission("users:admin") {
		return &apperr.Forbidden{Action: "revoke role", Resource: "user"}
	}

	if err := h.userRepo.RevokeRole(ctx, c.Param("id"), c.Param("role_id")); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// assignBranchRequest is the JSON body for POST /api/v1/users/:id/branches.
type assignBranchRequest struct {
	BranchID string `json:"branch_id"`
}

// AssignBranch handles POST /api/v1/users/:id/branches.
func (h *Handler) AssignBranch(c echo.Context) error {
	ctx := c.Request().Context()
	caller, ok := ctxutil.GetUser(ctx)
	if !ok || caller == nil {
		return &apperr.Unauthorized{}
	}
	if !caller.HasPermission("users:admin") {
		return &apperr.Forbidden{Action: "assign branch", Resource: "user"}
	}

	var req assignBranchRequest
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.BranchID == "" {
		return &apperr.Validation{Field: "branch_id", Message: "branch_id is required"}
	}

	if err := h.userRepo.AssignBranch(ctx, c.Param("id"), req.BranchID, caller.User.ID); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{"branch_id": req.BranchID})
}

// RevokeBranch handles DELETE /api/v1/users/:id/branches/:branch_id.
func (h *Handler) RevokeBranch(c echo.Context) error {
	ctx := c.Request().Context()
	caller, ok := ctxutil.GetUser(ctx)
	if !ok || caller == nil {
		return &apperr.Unauthorized{}
	}
	if !caller.HasPermission("users:admin") {
		return &apperr.Forbidden{Action: "revoke branch", Resource: "user"}
	}

	if err := h.userRepo.RevokeBranch(ctx, c.Param("id"), c.Param("branch_id")); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// userIntQueryParam reads an integer query parameter, returning defaultVal on absence or error.
func userIntQueryParam(c echo.Context, name string, defaultVal int) int {
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

// hashPassword returns a bcrypt hash of the given plaintext password.
func hashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// stepUpRequest is the JSON body for POST /api/v1/auth/stepup.
type stepUpRequest struct {
	Password string `json:"password"`
}

// Me handles GET /api/v1/auth/me.
// Returns the currently authenticated user with their roles and permissions.
// Used by the frontend to restore session state on page load.
func (h *Handler) Me(c echo.Context) error {
	user, ok := ctxutil.GetUser(c.Request().Context())
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	// Return UserWithRoles directly — serialises as { user: {...}, roles: [...], permissions: [...] }.
	return c.JSON(http.StatusOK, user)
}

// StepUp handles POST /api/v1/auth/stepup.
// Verifies the user's current password without creating a new session.
// Returns 200 {ok: true} on success, 401 on failure.
func (h *Handler) StepUp(c echo.Context) error {
	var req stepUpRequest
	if err := c.Bind(&req); err != nil {
		return &apperr.Validation{Field: "body", Message: "invalid JSON"}
	}
	if req.Password == "" {
		return &apperr.Validation{Field: "password", Message: "password is required"}
	}

	user, ok := ctxutil.GetUser(c.Request().Context())
	if !ok || user == nil {
		return &apperr.Unauthorized{}
	}
	session, _ := ctxutil.GetSession(c.Request().Context())
	sessionID := ""
	if session != nil {
		sessionID = session.ID
	}

	if err := h.service.StepUp(c.Request().Context(), user.User.ID, sessionID, req.Password); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
