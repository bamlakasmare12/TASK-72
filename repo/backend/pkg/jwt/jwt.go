package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID      int      `json:"user_id"`
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	RoleIDs     []int    `json:"role_ids"`
	Permissions []string `json:"permissions"`
	SessionID   string   `json:"session_id"`
	jwtlib.RegisteredClaims
}

var jwtSecret []byte

// InitJWTSecret is a no-op kept for backward compatibility. Use InitJWTSecretFromValue.
func InitJWTSecret() error {
	return InitJWTSecretFromValue("")
}

// InitJWTSecretFromValue initializes the JWT signing key from the provided string.
func InitJWTSecretFromValue(secret string) error {
	if secret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if len(secret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 characters")
	}
	jwtSecret = []byte(secret)
	return nil
}

// GenerateToken creates a signed JWT with user claims.
// The token itself is short-lived (30 min); session logic enforces idle/max lifetime separately.
func GenerateToken(userID int, username, sessionID string, roles []string, roleIDs []int, permissions []string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:      userID,
		Username:    username,
		Roles:       roles,
		RoleIDs:     roleIDs,
		Permissions: permissions,
		SessionID:   sessionID,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(now.Add(30 * time.Minute)),
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			Issuer:    "wlpr-portal",
		},
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken parses and validates a JWT string, returning its claims.
func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwtlib.ParseWithClaims(tokenString, &Claims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// RefreshToken issues a new token with the same claims but a fresh expiry.
func RefreshToken(claims *Claims) (string, error) {
	now := time.Now()
	claims.ExpiresAt = jwtlib.NewNumericDate(now.Add(30 * time.Minute))
	claims.IssuedAt = jwtlib.NewNumericDate(now)

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
