package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/services"
	"wlpr-portal/pkg/jwt"

	"github.com/labstack/echo/v4"
)

// SessionValidator is the minimal interface RequireAuth needs to validate a
// session. *services.AuthService satisfies this interface in production. A
// lightweight in-memory stub can satisfy it in unit tests, enabling full
// session-validation path coverage without a live database.
type SessionValidator interface {
	ValidateSession(ctx context.Context, sessionID string) (*models.Session, error)
}

type AuthMiddleware struct {
	sessionValidator SessionValidator
	configService    *services.ConfigService
}

// NewAuthMiddleware constructs an AuthMiddleware for production use.
// authService implements SessionValidator; passing it as the interface keeps
// the production call site in main.go unchanged.
func NewAuthMiddleware(sv SessionValidator, configService *services.ConfigService) *AuthMiddleware {
	return &AuthMiddleware{
		sessionValidator: sv,
		configService:    configService,
	}
}

// NewAuthMiddlewareWithConfig creates an AuthMiddleware without a session
// validator. JWT claims are trusted without a database lookup. Use this in
// tests that exercise RBAC or version-gate logic but do not need session
// validation.
func NewAuthMiddlewareWithConfig(configService *services.ConfigService) *AuthMiddleware {
	return &AuthMiddleware{configService: configService}
}

// NewAuthMiddlewareWithSessionAndConfig is a convenience alias that names the
// two-argument form explicitly — useful in tests to make the intent clear.
func NewAuthMiddlewareWithSessionAndConfig(sv SessionValidator, configService *services.ConfigService) *AuthMiddleware {
	return NewAuthMiddleware(sv, configService)
}

// RequireAuth validates the JWT, checks session validity (idle/max), and injects claims into context.
func (m *AuthMiddleware) RequireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization format")
			}

			claims, err := jwt.ValidateToken(parts[1])
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			// Validate session (idle timeout / max lifetime).
			// sessionValidator is nil only in tests that set up AuthMiddleware without
			// a validator (version-gate / RBAC-only tests). Full session path is
			// covered by tests that provide a mock SessionValidator.
			if m.sessionValidator != nil {
				session, err := m.sessionValidator.ValidateSession(c.Request().Context(), claims.SessionID)
				if err != nil || session == nil {
					return echo.NewHTTPError(http.StatusUnauthorized, "session expired")
				}
			}

			// Store claims in context for downstream handlers
			c.Set("claims", claims)
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("roles", claims.Roles)
			c.Set("role_ids", claims.RoleIDs)
			c.Set("permissions", claims.Permissions)
			c.Set("session_id", claims.SessionID)

			return next(c)
		}
	}
}

// RequireRole checks that the authenticated user has at least one of the specified roles.
// Returns 404 Not Found (not 403) so unauthorized users cannot discover that a feature exists.
func (m *AuthMiddleware) RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userRoles, ok := c.Get("roles").([]string)
			if !ok || len(userRoles) == 0 {
				return echo.NewHTTPError(http.StatusNotFound)
			}

			for _, required := range roles {
				for _, userRole := range userRoles {
					if required == userRole {
						return next(c)
					}
				}
			}

			// Feature invisibility: user without the role should not know the endpoint exists
			return echo.NewHTTPError(http.StatusNotFound)
		}
	}
}

// RequirePermission checks that the authenticated user has at least one of the specified permissions.
// Returns 404 Not Found (not 403) so unauthorized users cannot discover that a feature exists.
func (m *AuthMiddleware) RequirePermission(perms ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userPerms, ok := c.Get("permissions").([]string)
			if !ok || len(userPerms) == 0 {
				return echo.NewHTTPError(http.StatusNotFound)
			}

			for _, required := range perms {
				for _, userPerm := range userPerms {
					if required == userPerm {
						return next(c)
					}
				}
			}

			return echo.NewHTTPError(http.StatusNotFound)
		}
	}
}

// AppVersionCheck middleware checks the client's app version header and enforces compatibility.
// Clients below the minimum supported version enter a 14-day (configurable) read-only grace
// period. During grace, GET/HEAD/OPTIONS requests are allowed but writes are blocked.
// After the grace period expires, all requests are blocked.
// If no version header is provided, the client is treated as version "0.0.0" (oldest possible)
// so that the compatibility policy is still enforced.
func (m *AuthMiddleware) AppVersionCheck() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			clientVersion := c.Request().Header.Get("X-App-Version")

			minVersion, _ := m.configService.GetConfig("app.min_supported_version")
			if minVersion == "" {
				// No minimum version configured — skip enforcement
				return next(c)
			}

			if clientVersion == "" {
				// Missing version header: treat as oldest client ("0.0.0")
				// so the compatibility policy still applies.
				clientVersion = "0.0.0"
			}

			if CompareVersions(clientVersion, minVersion) < 0 {
				// Determine grace period: how long the min_version has been in effect
				graceDays := 14
				if val, ok := m.configService.GetConfig("app.read_only_grace_days"); ok {
					var d int
					if _, err := fmt.Sscanf(val, "%d", &d); err == nil && d > 0 {
						graceDays = d
					}
				}

				// Get the timestamp when the min_supported_version was last updated
				// (approximated by the config's updated_at). If not available, treat
				// as if just updated (full grace period).
				minVersionSetAt := m.configService.GetConfigUpdatedAt("app.min_supported_version")

				graceDeadline := minVersionSetAt.Add(time.Duration(graceDays) * 24 * time.Hour)
				withinGrace := time.Now().Before(graceDeadline)

				c.Response().Header().Set("X-App-Deprecated", "true")
				c.Response().Header().Set("X-Min-Version", minVersion)

				if !withinGrace {
					// Grace period expired: hard block ALL requests
					return echo.NewHTTPError(http.StatusUpgradeRequired,
						"client version unsupported and grace period expired; upgrade to "+minVersion+" or later")
				}

				// Within grace period: allow read-only access
				c.Set("read_only", true)
				daysLeft := int(time.Until(graceDeadline).Hours() / 24)
				c.Response().Header().Set("X-Grace-Days-Remaining", fmt.Sprintf("%d", daysLeft))

				method := c.Request().Method
				if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
					return echo.NewHTTPError(http.StatusUpgradeRequired,
						fmt.Sprintf("client version unsupported; read-only access for %d more days. Upgrade to %s or later.",
							daysLeft, minVersion))
				}
			}

			return next(c)
		}
	}
}

// CompareVersions compares semver strings. Returns -1, 0, or 1.
func CompareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var va, vb int
		if i < len(partsA) {
			for _, ch := range partsA[i] {
				if ch >= '0' && ch <= '9' {
					va = va*10 + int(ch-'0')
				}
			}
		}
		if i < len(partsB) {
			for _, ch := range partsB[i] {
				if ch >= '0' && ch <= '9' {
					vb = vb*10 + int(ch-'0')
				}
			}
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}
