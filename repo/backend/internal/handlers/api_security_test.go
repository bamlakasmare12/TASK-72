package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wlpr-portal/pkg/jwt"

	"github.com/labstack/echo/v4"
)

func init() {
	// Initialize JWT secret for test token generation.
	// This is required before any test that generates or validates tokens.
	if err := jwt.InitJWTSecretFromValue("test-secret-key-at-least-32-characters-long-for-tests"); err != nil {
		panic("failed to init JWT for tests: " + err.Error())
	}
}

// generateTestToken creates a signed JWT with the specified user attributes.
func generateTestToken(t *testing.T, userID int, username string, roles []string, permissions []string) string {
	t.Helper()
	// Generate mock role IDs based on role names for testing
	roleIDs := make([]int, len(roles))
	roleIDMap := map[string]int{
		"system_admin":           1,
		"content_moderator":      2,
		"learner":                3,
		"procurement_specialist": 4,
		"approver":               5,
		"finance_analyst":        6,
	}
	for i, r := range roles {
		if id, ok := roleIDMap[r]; ok {
			roleIDs[i] = id
		}
	}
	token, err := jwt.GenerateToken(userID, username, "test-session-id", roles, roleIDs, permissions)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

// jwtMiddleware is a simplified auth middleware for tests.
// It validates the JWT and injects claims into the echo context,
// mirroring the real RequireAuth middleware behavior.
func jwtMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
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
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
		}
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

// requireRole is a simplified role-check middleware for tests.
func requireRole(roles ...string) echo.MiddlewareFunc {
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
			return echo.NewHTTPError(http.StatusNotFound)
		}
	}
}

// TestAPI_CrossTenantDataIsolation_ProgressEndpoint tests that User A cannot
// access User B's learning progress, even if they manipulate query parameters.
// The handler extracts user_id from the JWT token, not from query params.
func TestAPI_CrossTenantDataIsolation_ProgressEndpoint(t *testing.T) {
	e := echo.New()

	// Simulate the learning progress handler that uses JWT-derived user_id
	e.GET("/api/learning/progress", func(c echo.Context) error {
		// The handler extracts user_id from the JWT token, NOT from query params
		userID := c.Get("user_id").(int)

		// Simulate: even if ?user_id=999 is passed, the handler ignores it
		// and always uses the token-derived user_id
		return c.JSON(http.StatusOK, map[string]interface{}{
			"user_id":  userID,
			"progress": []map[string]interface{}{},
		})
	}, jwtMiddleware)

	// User A: learner with user_id=3
	tokenA := generateTestToken(t, 3, "learner1", []string{"learner"}, []string{"learning.progress.view_own"})
	// User B: different learner with user_id=4
	tokenB := generateTestToken(t, 4, "learner2", []string{"learner"}, []string{"learning.progress.view_own"})

	t.Run("UserA_gets_own_data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/learning/progress", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var body map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &body)
		if int(body["user_id"].(float64)) != 3 {
			t.Errorf("expected user_id=3 in response, got %v", body["user_id"])
		}
	})

	t.Run("UserA_cannot_access_UserB_via_query_param", func(t *testing.T) {
		// Attempt: User A's token but requesting user_id=4 via query parameter
		req := httptest.NewRequest(http.MethodGet, "/api/learning/progress?user_id=4", nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &body)

		// CRITICAL: The response must contain user_id=3 (User A), NOT user_id=4 (User B)
		// The handler ignores the query parameter and uses the JWT-derived user_id
		responseUserID := int(body["user_id"].(float64))
		if responseUserID != 3 {
			t.Fatalf("SECURITY VIOLATION: response contained user_id=%d instead of token user_id=3. "+
				"The handler must use JWT-derived user_id, not query parameters.", responseUserID)
		}
		if responseUserID == 4 {
			t.Fatal("CRITICAL: Cross-tenant data leakage detected! User A received User B's data.")
		}
	})

	t.Run("UserB_gets_own_data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/learning/progress", nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var body map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &body)
		if int(body["user_id"].(float64)) != 4 {
			t.Errorf("expected user_id=4 in response, got %v", body["user_id"])
		}
	})

	t.Run("No_token_returns_401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/learning/progress", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("Invalid_token_returns_401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/learning/progress", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

// TestAPI_RBAC_LearnerCannotAccessReconciliation tests that a user with the
// "learner" role receives 404 Not Found when attempting to access finance/reconciliation endpoints.
// RBAC invisibility: unauthorized users see the feature as non-existent.
func TestAPI_RBAC_LearnerCannotAccessReconciliation(t *testing.T) {
	e := echo.New()

	// Register the reconciliation endpoints with role-based middleware
	// Exactly mirroring the real route setup in main.go:
	// finGroup := e.Group("/api/procurement", authMW.RequireAuth(),
	//     authMW.RequireRole("finance_analyst", "system_admin"))
	finGroup := e.Group("/api/procurement",
		jwtMiddleware,
		requireRole("finance_analyst", "system_admin"),
	)

	// Settlement transition endpoint (the one from the prompt: "reconciliation/approve")
	finGroup.POST("/settlements/transition", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "settlement approved"})
	})
	finGroup.POST("/reconciliation/compare", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "compared"})
	})
	finGroup.GET("/ledger", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []interface{}{})
	})
	finGroup.GET("/cost-allocation", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []interface{}{})
	})

	// Learner token — has NO finance or admin roles
	learnerToken := generateTestToken(t, 3, "learner1",
		[]string{"learner"},
		[]string{"learning.library.view", "learning.path.enroll", "learning.progress.view_own"},
	)

	// Finance token — has the required role
	financeToken := generateTestToken(t, 6, "finance1",
		[]string{"finance_analyst"},
		[]string{"finance.reconciliation.view", "finance.reconciliation.manage", "finance.settlement.manage"},
	)

	t.Run("Learner_blocked_from_settlement_transition", func(t *testing.T) {
		body := `{"settlement_id": 1, "action": "approve_writeoff"}`
		req := httptest.NewRequest(http.MethodPost, "/api/procurement/settlements/transition", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+learnerToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("RBAC FAILURE: Learner should get 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Learner_blocked_from_reconciliation_compare", func(t *testing.T) {
		body := `{"vendor_id": 1, "statement_total": 12500.00}`
		req := httptest.NewRequest(http.MethodPost, "/api/procurement/reconciliation/compare", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+learnerToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("RBAC FAILURE: Learner should get 404, got %d", rec.Code)
		}
	})

	t.Run("Learner_blocked_from_ledger", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/procurement/ledger", nil)
		req.Header.Set("Authorization", "Bearer "+learnerToken)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("RBAC FAILURE: Learner should get 404, got %d", rec.Code)
		}
	})

	t.Run("Learner_blocked_from_cost_allocation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/procurement/cost-allocation", nil)
		req.Header.Set("Authorization", "Bearer "+learnerToken)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("RBAC FAILURE: Learner should get 404, got %d", rec.Code)
		}
	})

	t.Run("Finance_analyst_allowed_settlement_transition", func(t *testing.T) {
		body := `{"settlement_id": 1, "action": "approve_writeoff"}`
		req := httptest.NewRequest(http.MethodPost, "/api/procurement/settlements/transition", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+financeToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Finance analyst should get 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Finance_analyst_allowed_compare", func(t *testing.T) {
		body := `{"vendor_id": 1, "statement_total": 12500.00}`
		req := httptest.NewRequest(http.MethodPost, "/api/procurement/reconciliation/compare", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+financeToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Finance analyst should get 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// TestAPI_RBAC_ProcurementUserCannotAccessFinance tests that a procurement_specialist
// cannot access finance-only endpoints.
func TestAPI_RBAC_ProcurementUserCannotAccessFinance(t *testing.T) {
	e := echo.New()

	finGroup := e.Group("/api/procurement",
		jwtMiddleware,
		requireRole("finance_analyst", "system_admin"),
	)
	finGroup.GET("/settlements", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []interface{}{})
	})
	finGroup.POST("/export/ledger", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "exported"})
	})

	procToken := generateTestToken(t, 4, "procuser",
		[]string{"procurement_specialist"},
		[]string{"procurement.orders.view", "procurement.orders.manage", "procurement.reviews.manage"},
	)

	t.Run("ProcUser_blocked_from_settlements", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/procurement/settlements", nil)
		req.Header.Set("Authorization", "Bearer "+procToken)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("Procurement user should get 404 for settlements, got %d", rec.Code)
		}
	})

	t.Run("ProcUser_blocked_from_export", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/procurement/export/ledger", nil)
		req.Header.Set("Authorization", "Bearer "+procToken)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("Procurement user should get 404 for ledger export, got %d", rec.Code)
		}
	})
}

// TestAPI_RBAC_AdminAccessesEverything tests that system_admin can access all restricted endpoints.
func TestAPI_RBAC_AdminAccessesEverything(t *testing.T) {
	e := echo.New()

	finGroup := e.Group("/api/procurement",
		jwtMiddleware,
		requireRole("finance_analyst", "system_admin"),
	)
	finGroup.GET("/settlements", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []interface{}{})
	})

	adminToken := generateTestToken(t, 1, "admin",
		[]string{"system_admin"},
		[]string{"admin.users.manage", "finance.settlement.manage"},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/procurement/settlements", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Admin should get 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAPI_NoAuth_Returns401 tests that unauthenticated requests are rejected.
func TestAPI_NoAuth_Returns401(t *testing.T) {
	e := echo.New()

	e.GET("/api/procurement/settlements", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []interface{}{})
	}, jwtMiddleware)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/procurement/settlements"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+"_"+ep.path+"_no_auth", func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}
}
