package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/repository"
	"wlpr-portal/internal/services"
	"wlpr-portal/pkg/crypto"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	authService *services.AuthService
	mfaService  *services.MFAService
	userRepo    *repository.UserRepository
}

func NewAuthHandler(authService *services.AuthService, mfaService *services.MFAService, userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		mfaService:  mfaService,
		userRepo:    userRepo,
	}
}

// POST /api/auth/login
func (h *AuthHandler) Login(c echo.Context) error {
	var req models.LoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Username == "" || req.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "username and password are required")
	}

	resp, err := h.authService.Login(c.Request().Context(), req, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	return c.JSON(http.StatusOK, resp)
}

// POST /api/auth/mfa/verify
func (h *AuthHandler) VerifyMFA(c echo.Context) error {
	var req models.MFAVerifyRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Code == "" || req.SessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code and session_id are required")
	}

	resp, err := h.authService.VerifyMFA(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid MFA code")
	}

	return c.JSON(http.StatusOK, resp)
}

// POST /api/auth/mfa/setup  (requires auth)
func (h *AuthHandler) SetupMFA(c echo.Context) error {
	userID := c.Get("user_id").(int)
	username := c.Get("username").(string)

	secret, provURI, err := h.mfaService.SetupMFA(c.Request().Context(), userID, username)
	if err != nil {
		log.Printf("[auth] MFA setup error user=%d: %v", userID, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to set up MFA")
	}

	return c.JSON(http.StatusOK, models.MFASetupResponse{
		Secret: secret,
		QRCode: provURI,
	})
}

// POST /api/auth/mfa/confirm  (requires auth)
func (h *AuthHandler) ConfirmMFA(c echo.Context) error {
	userID := c.Get("user_id").(int)

	var body struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&body); err != nil || body.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code is required")
	}

	recoveryCodes, err := h.mfaService.ConfirmMFA(c.Request().Context(), userID, body.Code)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid code or MFA not initiated")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":        "MFA enabled successfully",
		"recovery_codes": recoveryCodes,
	})
}

// POST /api/auth/mfa/disable  (requires auth)
func (h *AuthHandler) DisableMFA(c echo.Context) error {
	userID := c.Get("user_id").(int)

	if err := h.mfaService.DisableMFA(c.Request().Context(), userID); err != nil {
		log.Printf("[auth] MFA disable error user=%d: %v", userID, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to disable MFA")
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "MFA disabled"})
}

// POST /api/auth/logout  (requires auth)
func (h *AuthHandler) Logout(c echo.Context) error {
	sessionID := c.Get("session_id").(string)
	userID := c.Get("user_id").(int)

	if err := h.authService.Logout(c.Request().Context(), sessionID, userID, c.RealIP()); err != nil {
		log.Printf("[auth] logout error user=%d: %v", userID, err)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "logged out"})
}

// GET /api/auth/me  (requires auth)
func (h *AuthHandler) Me(c echo.Context) error {
	claims := c.Get("claims")
	return c.JSON(http.StatusOK, claims)
}

// POST /api/auth/register  (public)
// The first user to register is auto-assigned system_admin (bootstrap).
// All subsequent users may only self-select non-privileged roles.
// Privileged roles (system_admin) are assigned by an admin via /api/admin/users/assign-role.
func (h *AuthHandler) Register(c echo.Context) error {
	var req models.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Email) == "" ||
		strings.TrimSpace(req.Password) == "" || strings.TrimSpace(req.DisplayName) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "username, email, password, and display_name are required")
	}
	if len(req.Password) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest, "password must be at least 8 characters")
	}

	ctx := c.Request().Context()

	// Check if this is the very first user (bootstrap: auto-assign system_admin)
	userCount, err := h.userRepo.CountUsers(ctx)
	if err != nil {
		log.Printf("[auth] count users error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "registration temporarily unavailable")
	}
	isFirstUser := userCount == 0

	var assignedRole string
	if isFirstUser {
		// First user bootstrap: ignore the requested role and assign system_admin
		assignedRole = "system_admin"
	} else {
		// Subsequent users: validate role from the safe self-registration set
		if strings.TrimSpace(req.Role) == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "role is required")
		}
		if !models.ValidRoles()[req.Role] {
			return echo.NewHTTPError(http.StatusBadRequest,
				"invalid role; choose one of: learner, procurement_specialist, approver, finance_analyst, content_moderator")
		}
		assignedRole = req.Role
	}

	existing, err := h.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		log.Printf("[auth] find user error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "registration temporarily unavailable")
	}
	if existing != nil {
		return echo.NewHTTPError(http.StatusConflict, "username already taken")
	}

	hashedPassword, err := crypto.HashPassword(req.Password)
	if err != nil {
		log.Printf("[auth] hash error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "registration temporarily unavailable")
	}

	user, err := h.userRepo.CreateUser(ctx, req.Username, req.Email, hashedPassword,
		req.DisplayName, req.JobFamily, req.Department, req.CostCenter)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			if strings.Contains(err.Error(), "email") {
				return echo.NewHTTPError(http.StatusConflict, "email already registered")
			}
			return echo.NewHTTPError(http.StatusConflict, "username already taken")
		}
		log.Printf("[auth] create user error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "registration temporarily unavailable")
	}

	if err := h.userRepo.AssignRole(ctx, user.ID, assignedRole, nil); err != nil {
		log.Printf("[auth] assign role error user=%d role=%s: %v", user.ID, assignedRole, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "registration succeeded but role assignment failed; contact an administrator")
	}

	uwr, err := h.userRepo.GetUserWithRoles(ctx, user.ID)
	if err != nil {
		uwr = &models.UserWithRoles{User: *user}
	}

	msg := fmt.Sprintf("Registration successful. You have been assigned the %s role.",
		strings.ReplaceAll(assignedRole, "_", " "))
	if isFirstUser {
		msg = "Welcome! As the first user, you have been assigned the system admin role."
	}

	return c.JSON(http.StatusCreated, models.RegisterResponse{
		Message: msg,
		User:    uwr,
	})
}

// GET /api/admin/users  (admin only)
func (h *AuthHandler) ListUsers(c echo.Context) error {
	users, err := h.userRepo.GetAllUsers(c.Request().Context())
	if err != nil {
		log.Printf("[admin] list users error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve users")
	}
	return c.JSON(http.StatusOK, users)
}

// POST /api/admin/users/assign-role  (admin only)
func (h *AuthHandler) AssignRole(c echo.Context) error {
	var req models.AssignRoleRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.UserID == 0 || req.Role == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id and role are required")
	}

	if !models.AllRoles()[req.Role] {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role name")
	}

	adminID := c.Get("user_id").(int)
	if err := h.userRepo.AssignRole(c.Request().Context(), req.UserID, req.Role, &adminID); err != nil {
		log.Printf("[admin] assign role error user=%d role=%s: %v", req.UserID, req.Role, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to assign role")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": fmt.Sprintf("role '%s' assigned to user %d", req.Role, req.UserID),
	})
}
