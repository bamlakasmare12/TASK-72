package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/repository"
	"wlpr-portal/pkg/crypto"
	"wlpr-portal/pkg/jwt"

	"github.com/google/uuid"
)

type AuthService struct {
	userRepo   *repository.UserRepository
	configRepo *repository.ConfigRepository
}

func NewAuthService(userRepo *repository.UserRepository, configRepo *repository.ConfigRepository) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		configRepo: configRepo,
	}
}

// Login performs credential validation and returns either a token or an MFA challenge.
func (s *AuthService) Login(ctx context.Context, req models.LoginRequest, ipAddress, userAgent string) (*models.LoginResponse, error) {
	user, err := s.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if !user.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, fmt.Errorf("account is locked until %s", user.LockedUntil.Format(time.RFC3339))
	}

	if !crypto.CheckPassword(req.Password, user.PasswordHash) {
		_ = s.userRepo.IncrementFailedLogin(ctx, user.ID)
		return nil, fmt.Errorf("invalid credentials")
	}

	_ = s.userRepo.ResetFailedLogin(ctx, user.ID)

	// Get session timeout configs
	idleTimeout := 900  // 15 min default
	maxLifetime := 28800 // 8 hours default
	if cfg, err := s.configRepo.GetConfigByKey(ctx, "session.idle_timeout_seconds"); err == nil && cfg != nil {
		fmt.Sscanf(cfg.Value, "%d", &idleTimeout)
	}
	if cfg, err := s.configRepo.GetConfigByKey(ctx, "session.max_lifetime_seconds"); err == nil && cfg != nil {
		fmt.Sscanf(cfg.Value, "%d", &maxLifetime)
	}

	now := time.Now()
	sessionID := uuid.New().String()

	session := &models.Session{
		ID:           sessionID,
		UserID:       user.ID,
		TokenHash:    "", // will be set after token generation
		Status:       "active",
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		CreatedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now.Add(time.Duration(maxLifetime) * time.Second),
		IdleTimeoutS: idleTimeout,
		MaxLifetimeS: maxLifetime,
	}

	if user.MFAEnabled {
		// Create a pending session; token is not issued yet
		session.TokenHash = "pending_mfa"
		if err := s.userRepo.CreateSession(ctx, session); err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		return &models.LoginResponse{
			RequiresMFA: true,
			SessionID:   sessionID,
		}, nil
	}

	// No MFA: issue token directly
	return s.issueToken(ctx, user, session)
}

// VerifyMFA validates TOTP code and issues a token for a pending MFA session.
func (s *AuthService) VerifyMFA(ctx context.Context, req models.MFAVerifyRequest) (*models.LoginResponse, error) {
	session, err := s.userRepo.GetSession(ctx, req.SessionID)
	if err != nil || session == nil {
		return nil, fmt.Errorf("invalid session")
	}
	if session.TokenHash != "pending_mfa" {
		return nil, fmt.Errorf("session is not pending MFA")
	}

	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	if !user.MFAEnabled || user.MFASecretEnc == nil {
		return nil, fmt.Errorf("MFA not configured for this user")
	}

	secret, err := crypto.DecryptString(user.MFASecretEnc)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt MFA secret")
	}

	if !ValidateTOTP(secret, req.Code) {
		return nil, fmt.Errorf("invalid MFA code")
	}

	return s.issueToken(ctx, user, session)
}

func (s *AuthService) issueToken(ctx context.Context, user *models.User, session *models.Session) (*models.LoginResponse, error) {
	roles, err := s.userRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch roles: %w", err)
	}

	perms, err := s.userRepo.GetUserPermissions(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch permissions: %w", err)
	}

	roleNames := make([]string, len(roles))
	roleIDs := make([]int, len(roles))
	for i, r := range roles {
		roleNames[i] = r.Name
		roleIDs[i] = r.ID
	}

	token, err := jwt.GenerateToken(user.ID, user.Username, session.ID, roleNames, roleIDs, perms)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Store token hash in session
	h := sha256.Sum256([]byte(token))
	session.TokenHash = hex.EncodeToString(h[:])
	if err := s.userRepo.CreateSession(ctx, session); err != nil {
		// If session already exists (MFA flow), update token hash
		_ = s.userRepo.TouchSession(ctx, session.ID)
	}

	_ = s.userRepo.LogAudit(ctx, user.ID, "login", "auth", "user", user.ID, session.IPAddress)

	return &models.LoginResponse{
		Token:       token,
		RequiresMFA: false,
		User: &models.UserWithRoles{
			User:        *user,
			Roles:       roles,
			Permissions: perms,
		},
	}, nil
}

// Logout revokes the session.
func (s *AuthService) Logout(ctx context.Context, sessionID string, userID int, ip string) error {
	_ = s.userRepo.RevokeSession(ctx, sessionID)
	_ = s.userRepo.LogAudit(ctx, userID, "logout", "auth", "session", 0, ip)
	return nil
}

// ValidateSession checks that the session is still active, not idle, not expired.
func (s *AuthService) ValidateSession(ctx context.Context, sessionID string) (*models.Session, error) {
	session, err := s.userRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}
	if !repository.IsSessionValid(session) {
		_ = s.userRepo.RevokeSession(ctx, sessionID)
		return nil, fmt.Errorf("session expired")
	}
	_ = s.userRepo.TouchSession(ctx, sessionID)
	return session, nil
}
