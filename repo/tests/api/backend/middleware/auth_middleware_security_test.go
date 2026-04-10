package middleware_test

// auth_middleware_security_test.go
//
// Production-wired integration tests for the real AuthMiddleware stack.
// These tests exercise RequireAuth -> RequireRole -> AppVersionCheck using
// real JWT generation/validation and real ConfigService (in-memory, no DB).
// They guard against the security regressions identified in M-03:
//   - Missing/invalid version identity must not bypass the compatibility policy
//   - Auth + RBAC middleware must be exercised by non-stub integration tests
//   - Role-page orchestration (learner blocked from finance, cross-role visibility)

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"wlpr-portal/internal/middleware"
	"wlpr-portal/internal/models"
	"wlpr-portal/internal/services"
	"wlpr-portal/pkg/jwt"

	"github.com/labstack/echo/v4"
)

func init() {
	// Shared JWT secret for this test file.
	if err := jwt.InitJWTSecretFromValue("test-secret-key-at-least-32-characters-for-security"); err != nil {
		panic("failed to init JWT secret in security tests: " + err.Error())
	}
}

// makeToken generates a signed test JWT.
func makeToken(t *testing.T, userID int, sessionID string, roles []string, perms []string) string {
	t.Helper()
	roleIDs := make([]int, len(roles))
	roleMap := map[string]int{
		"system_admin": 1, "content_moderator": 2, "learner": 3,
		"procurement_specialist": 4, "approver": 5, "finance_analyst": 6,
	}
	for i, r := range roles {
		if id, ok := roleMap[r]; ok {
			roleIDs[i] = id
		}
	}
	tok, err := jwt.GenerateToken(userID, "testuser", sessionID, roles, roleIDs, perms)
	if err != nil {
		t.Fatalf("makeToken: %v", err)
	}
	return tok
}

// -----------------------------------------------------------------------
// H-02: Version gate enforcement with the real AppVersionCheck middleware
// -----------------------------------------------------------------------

// TestRealVersionGate_MissingHeader_WithMinVersion ensures that when the
// X-App-Version header is absent and a minimum version is configured, the
// request is subject to the compatibility policy (treated as version "0.0.0").
func TestRealVersionGate_MissingHeader_WithMinVersion_GETAllowed_InGrace(t *testing.T) {
	// min version set 5 days ago, 14-day grace -> within grace
	configs := map[string]models.Config{
		"app.min_supported_version": {
			Key:       "app.min_supported_version",
			Value:     "2.0.0",
			UpdatedAt: time.Now().Add(-5 * 24 * time.Hour),
		},
	}
	configSvc := services.NewConfigServiceForTest(configs)
	mw := middleware.NewAuthMiddlewareWithConfig(configSvc)

	e := echo.New()
	e.Use(mw.AppVersionCheck())
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No X-App-Version header
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("missing header within grace should allow GET, got %d", rec.Code)
	}
	if rec.Header().Get("X-App-Deprecated") != "true" {
		t.Error("expected X-App-Deprecated: true when version header is missing but min_version is set")
	}
}

func TestRealVersionGate_MissingHeader_WithMinVersion_POSTBlocked_InGrace(t *testing.T) {
	configs := map[string]models.Config{
		"app.min_supported_version": {
			Key:       "app.min_supported_version",
			Value:     "2.0.0",
			UpdatedAt: time.Now().Add(-5 * 24 * time.Hour),
		},
	}
	configSvc := services.NewConfigServiceForTest(configs)
	mw := middleware.NewAuthMiddlewareWithConfig(configSvc)

	e := echo.New()
	e.Use(mw.AppVersionCheck())
	e.POST("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("missing header + min version configured + within grace: POST must be blocked (426), got %d", rec.Code)
	}
}

func TestRealVersionGate_MissingHeader_GraceExpired_AllBlocked(t *testing.T) {
	configs := map[string]models.Config{
		"app.min_supported_version": {
			Key:       "app.min_supported_version",
			Value:     "2.0.0",
			UpdatedAt: time.Now().Add(-20 * 24 * time.Hour), // 20 days ago
		},
	}
	configSvc := services.NewConfigServiceForTest(configs)
	mw := middleware.NewAuthMiddlewareWithConfig(configSvc)

	e := echo.New()
	e.Use(mw.AppVersionCheck())
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "should not reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("missing header + grace expired: GET must be blocked (426), got %d", rec.Code)
	}
}

func TestRealVersionGate_NoMinVersionConfigured_AnyHeaderPasses(t *testing.T) {
	// No min_supported_version in config -> no enforcement
	configs := map[string]models.Config{}
	configSvc := services.NewConfigServiceForTest(configs)
	mw := middleware.NewAuthMiddlewareWithConfig(configSvc)

	e := echo.New()
	e.Use(mw.AppVersionCheck())
	e.POST("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	for _, header := range []string{"", "0.0.1", "99.0.0"} {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		if header != "" {
			req.Header.Set("X-App-Version", header)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("version=%q with no min config should pass, got %d", header, rec.Code)
		}
	}
}

func TestRealVersionGate_CurrentVersion_NotDeprecated(t *testing.T) {
	configs := map[string]models.Config{
		"app.min_supported_version": {
			Key:       "app.min_supported_version",
			Value:     "2.0.0",
			UpdatedAt: time.Now(),
		},
	}
	configSvc := services.NewConfigServiceForTest(configs)
	mw := middleware.NewAuthMiddlewareWithConfig(configSvc)

	e := echo.New()
	e.Use(mw.AppVersionCheck())
	e.POST("/test", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-App-Version", "2.1.0") // current version
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("current version should not be blocked, got %d", rec.Code)
	}
	if rec.Header().Get("X-App-Deprecated") == "true" {
		t.Error("current version should not be flagged as deprecated")
	}
}

// -----------------------------------------------------------------------
// H-01: RequireRole middleware blocks the correct roles from finance endpoints
// -----------------------------------------------------------------------

// TestRealRequireRole_LearnerBlockedFromFinance tests that a learner gets 404
// from a finance-only endpoint using the full RequireAuth + RequireRole chain.
// authService is nil so the DB session check is skipped; JWT signature is
// still validated by the real RequireAuth code path.
func TestRealRequireRole_LearnerBlockedFromFinance(t *testing.T) {
	configSvc := services.NewConfigServiceForTest(map[string]models.Config{})
	mw := middleware.NewAuthMiddlewareWithConfig(configSvc) // nil sessionValidator: JWT trusted, no DB needed

	e := echo.New()
	financeGroup := e.Group("/api/procurement",
		mw.RequireAuth(),
		mw.RequireRole("finance_analyst", "system_admin"),
	)
	financeGroup.GET("/settlements", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []interface{}{})
	})
	financeGroup.POST("/reconciliation/compare", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	learnerToken := makeToken(t, 3, "test-session-id",
		[]string{"learner"},
		[]string{"learning.library.view"},
	)
	financeToken := makeToken(t, 6, "test-session-id",
		[]string{"finance_analyst"},
		[]string{"finance.reconciliation.view"},
	)

	tests := []struct {
		name       string
		token      string
		method     string
		path       string
		wantStatus int
	}{
		{"learner_blocked_settlements_GET", learnerToken, "GET", "/api/procurement/settlements", http.StatusNotFound},
		{"learner_blocked_compare_POST", learnerToken, "POST", "/api/procurement/reconciliation/compare", http.StatusNotFound},
		{"finance_allowed_settlements_GET", financeToken, "GET", "/api/procurement/settlements", http.StatusOK},
		{"finance_allowed_compare_POST", financeToken, "POST", "/api/procurement/reconciliation/compare", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("want %d, got %d", tc.wantStatus, rec.Code)
			}
		})
	}
}

// TestRealRequireRole_AllSixRoles verifies each of the six roles against
// a finance-restricted endpoint using the full RequireAuth + RequireRole chain.
func TestRealRequireRole_AllSixRoles_FinanceEndpoint(t *testing.T) {
	configSvc := services.NewConfigServiceForTest(map[string]models.Config{})
	mw := middleware.NewAuthMiddlewareWithConfig(configSvc)

	e := echo.New()
	financeGroup := e.Group("/api/procurement",
		mw.RequireAuth(),
		mw.RequireRole("finance_analyst", "system_admin"),
	)
	financeGroup.GET("/ledger", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []interface{}{})
	})

	roleExpectations := []struct {
		role        string
		wantAllowed bool
	}{
		{"system_admin", true},
		{"finance_analyst", true},
		{"learner", false},
		{"content_moderator", false},
		{"procurement_specialist", false},
		{"approver", false},
	}

	for _, tc := range roleExpectations {
		t.Run("role_"+tc.role, func(t *testing.T) {
			tok := makeToken(t, 1, "test-session-id", []string{tc.role}, []string{})
			req := httptest.NewRequest(http.MethodGet, "/api/procurement/ledger", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if tc.wantAllowed && rec.Code != http.StatusOK {
				t.Errorf("role %s should be allowed, got %d", tc.role, rec.Code)
			}
			if !tc.wantAllowed && rec.Code != http.StatusNotFound {
				t.Errorf("role %s should be blocked (404), got %d", tc.role, rec.Code)
			}
		})
	}
}

// TestRealRequireRole_DisputeEndpoint tests multi-role access to dispute endpoints
// using the full RequireAuth + RequireRole middleware chain.
func TestRealRequireRole_DisputeEndpoint_MultiRole(t *testing.T) {
	configSvc := services.NewConfigServiceForTest(map[string]models.Config{})
	mw := middleware.NewAuthMiddlewareWithConfig(configSvc)

	e := echo.New()
	disputeGroup := e.Group("/api/procurement",
		mw.RequireAuth(),
		mw.RequireRole("procurement_specialist", "content_moderator", "approver", "system_admin"),
	)
	disputeGroup.GET("/disputes", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []interface{}{})
	})

	roleExpectations := []struct {
		role        string
		wantAllowed bool
	}{
		{"system_admin", true},
		{"procurement_specialist", true},
		{"content_moderator", true},
		{"approver", true},
		{"learner", false},
		{"finance_analyst", false},
	}

	for _, tc := range roleExpectations {
		t.Run("role_"+tc.role, func(t *testing.T) {
			tok := makeToken(t, 1, "test-session-id", []string{tc.role}, []string{})
			req := httptest.NewRequest(http.MethodGet, "/api/procurement/disputes", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if tc.wantAllowed && rec.Code != http.StatusOK {
				t.Errorf("role %s should access disputes, got %d", tc.role, rec.Code)
			}
			if !tc.wantAllowed && rec.Code != http.StatusNotFound {
				t.Errorf("role %s should be blocked from disputes (404), got %d", tc.role, rec.Code)
			}
		})
	}
}

// TestRealVersionGate_AndRBAC_Combined tests version check + RBAC in the same
// middleware chain — a learner with an old version seeing a finance endpoint.
func TestRealVersionGate_AndRBAC_Combined(t *testing.T) {
	// Grace period active (min set 5 days ago)
	configs := map[string]models.Config{
		"app.min_supported_version": {
			Key:       "app.min_supported_version",
			Value:     "2.0.0",
			UpdatedAt: time.Now().Add(-5 * 24 * time.Hour),
		},
	}
	configSvc := services.NewConfigServiceForTest(configs)
	mw := middleware.NewAuthMiddlewareWithConfig(configSvc)

	e := echo.New()
	e.Use(mw.AppVersionCheck())

	finGroup := e.Group("/api/procurement",
		mw.RequireAuth(),
		mw.RequireRole("finance_analyst", "system_admin"),
	)
	finGroup.GET("/settlements", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []interface{}{})
	})

	learnerToken := makeToken(t, 3, "test-session-id", []string{"learner"}, []string{})

	t.Run("learner_old_version_blocked_by_RBAC_first", func(t *testing.T) {
		// Even with old version (within grace), RBAC blocks learner -> 404
		req := httptest.NewRequest(http.MethodGet, "/api/procurement/settlements", nil)
		req.Header.Set("Authorization", "Bearer "+learnerToken)
		req.Header.Set("X-App-Version", "1.0.0")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("learner should be RBAC blocked (404), got %d", rec.Code)
		}
	})

	t.Run("no_auth_blocked_by_401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/procurement/settlements", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		// No auth header -> 401 from the auth middleware in the group
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("no auth should return 401, got %d", rec.Code)
		}
	})
}

// -----------------------------------------------------------------------
// M-03: Full RequireAuth session-validation chain with a mock SessionValidator
//
// These tests wire a real *AuthMiddleware with a non-nil SessionValidator so
// the session-check branch inside RequireAuth is always executed. They cover:
//   - active session  → request passes through to the handler
//   - missing session → 401 Unauthorized
//   - revoked/expired session (nil return) → 401 Unauthorized
//   - JWT for unknown session ID → 401 Unauthorized
// -----------------------------------------------------------------------

// stubSessionStore is an in-memory SessionValidator used to test the
// session-validation path of RequireAuth without a live database.
type stubSessionStore struct {
	sessions map[string]*models.Session
}

func (s *stubSessionStore) ValidateSession(_ context.Context, sessionID string) (*models.Session, error) {
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return sess, nil
}

// newStubSession returns a minimal active session for the given session ID.
func newStubSession(sessionID string, userID int) *models.Session {
	return &models.Session{
		ID:     sessionID,
		UserID: userID,
		Status: "active",
	}
}

// buildMWWithSession constructs a real AuthMiddleware backed by the stub store.
func buildMWWithSession(sessions map[string]*models.Session) *middleware.AuthMiddleware {
	return middleware.NewAuthMiddlewareWithSessionAndConfig(
		&stubSessionStore{sessions: sessions},
		services.NewConfigServiceForTest(map[string]models.Config{}),
	)
}

func TestRequireAuth_ActiveSession_Passes(t *testing.T) {
	const sessID = "active-session-abc"
	mw := buildMWWithSession(map[string]*models.Session{
		sessID: newStubSession(sessID, 7),
	})

	e := echo.New()
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, mw.RequireAuth())

	tok := makeToken(t, 7, sessID, []string{"finance_analyst"}, []string{})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("active session should pass, got %d", rec.Code)
	}
}

func TestRequireAuth_MissingSession_Returns401(t *testing.T) {
	// Store is empty — no sessions registered
	mw := buildMWWithSession(map[string]*models.Session{})

	e := echo.New()
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, mw.RequireAuth())

	tok := makeToken(t, 7, "nonexistent-session", []string{"finance_analyst"}, []string{})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing session should return 401, got %d", rec.Code)
	}
}

func TestRequireAuth_RevokedSession_Returns401(t *testing.T) {
	// Override with a validator that always returns nil session
	mw := middleware.NewAuthMiddlewareWithSessionAndConfig(
		&revokedSessionStore{},
		services.NewConfigServiceForTest(map[string]models.Config{}),
	)

	e := echo.New()
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, mw.RequireAuth())

	tok := makeToken(t, 7, "revoked-session", []string{"finance_analyst"}, []string{})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session should return 401, got %d", rec.Code)
	}
}

// revokedSessionStore always returns a nil session (simulates revoked / expired).
type revokedSessionStore struct{}

func (r *revokedSessionStore) ValidateSession(_ context.Context, _ string) (*models.Session, error) {
	return nil, nil // nil session triggers the "session expired" 401
}

func TestRequireAuth_NoAuthHeader_Returns401(t *testing.T) {
	mw := buildMWWithSession(map[string]*models.Session{})

	e := echo.New()
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, mw.RequireAuth())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth header should return 401, got %d", rec.Code)
	}
}

func TestRequireAuth_InvalidJWT_Returns401(t *testing.T) {
	mw := buildMWWithSession(map[string]*models.Session{})

	e := echo.New()
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, mw.RequireAuth())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not.a.real.token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid JWT should return 401, got %d", rec.Code)
	}
}

func TestRequireAuth_ActiveSession_ClaimsInjectedIntoContext(t *testing.T) {
	// Verify that user_id, roles, permissions are all set after successful auth
	const sessID = "claims-check-session"
	mw := buildMWWithSession(map[string]*models.Session{
		sessID: newStubSession(sessID, 42),
	})

	var capturedUserID interface{}
	var capturedRoles interface{}

	e := echo.New()
	e.GET("/me", func(c echo.Context) error {
		capturedUserID = c.Get("user_id")
		capturedRoles = c.Get("roles")
		return c.String(http.StatusOK, "ok")
	}, mw.RequireAuth())

	tok := makeToken(t, 42, sessID, []string{"system_admin"}, []string{"admin.all"})
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedUserID != 42 {
		t.Errorf("user_id in context: want 42, got %v", capturedUserID)
	}
	roles, ok := capturedRoles.([]string)
	if !ok || len(roles) == 0 || roles[0] != "system_admin" {
		t.Errorf("roles in context: want [system_admin], got %v", capturedRoles)
	}
}
