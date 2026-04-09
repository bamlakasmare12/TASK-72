package models

import "time"

type User struct {
	ID               int        `json:"id"`
	Username         string     `json:"username"`
	Email            string     `json:"email"`
	PasswordHash     string     `json:"-"`
	DisplayName      string     `json:"display_name"`
	MFAEnabled       bool       `json:"mfa_enabled"`
	MFASecretEnc     []byte     `json:"-"`
	MFARecoveryEnc   []byte     `json:"-"`
	JobFamily        *string    `json:"job_family,omitempty"`
	Department       *string    `json:"department,omitempty"`
	CostCenter       *string    `json:"cost_center,omitempty"`
	IsActive         bool       `json:"is_active"`
	FailedLoginCount int        `json:"-"`
	LockedUntil      *time.Time `json:"-"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type UserWithRoles struct {
	User
	Roles       []Role   `json:"roles"`
	Permissions []string `json:"permissions"`
}

type Role struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Session struct {
	ID            string    `json:"id"`
	UserID        int       `json:"user_id"`
	TokenHash     string    `json:"-"`
	Status        string    `json:"status"`
	IPAddress     string    `json:"ip_address,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	LastActiveAt  time.Time `json:"last_active_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	IdleTimeoutS  int       `json:"idle_timeout_s"`
	MaxLifetimeS  int       `json:"max_lifetime_s"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type MFAVerifyRequest struct {
	Code      string `json:"code" validate:"required"`
	SessionID string `json:"session_id" validate:"required"`
}

type LoginResponse struct {
	Token       string        `json:"token,omitempty"`
	RequiresMFA bool          `json:"requires_mfa"`
	SessionID   string        `json:"session_id,omitempty"`
	User        *UserWithRoles `json:"user,omitempty"`
}

type MFASetupResponse struct {
	Secret string `json:"secret"`
	QRCode string `json:"qr_code"`
}
