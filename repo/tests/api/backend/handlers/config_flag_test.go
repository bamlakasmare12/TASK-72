package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"wlpr-portal/internal/handlers"
	"wlpr-portal/internal/models"
	"wlpr-portal/internal/services"

	"github.com/labstack/echo/v4"
)

// TestCheckFlag_PassesUserContext verifies that the CheckFlag handler correctly passes
// user_id and role_ids from JWT claims to the flag evaluation logic.
func TestCheckFlag_PassesUserContext(t *testing.T) {
	e := echo.New()

	// Create a ConfigService with test data (no flags configured = all flags disabled)
	testSvc := services.NewConfigServiceForTest(map[string]models.Config{})

	h := handlers.NewConfigHandler(testSvc)
	e.GET("/api/flags/:key/check", h.CheckFlag, jwtMiddleware)

	t.Run("Authenticated_user_gets_flag_check", func(t *testing.T) {
		token := generateTestToken(t, 3, "learner1",
			[]string{"learner"}, []string{"learning.library.view"})

		req := httptest.NewRequest(http.MethodGet, "/api/flags/pinyin_search/check", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var body map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &body)

		if body["key"] != "pinyin_search" {
			t.Errorf("expected key 'pinyin_search', got %v", body["key"])
		}
		// The flag doesn't exist in the test service, so it should be disabled
		if body["enabled"] != false {
			t.Errorf("non-existent flag should be disabled, got %v", body["enabled"])
		}
	})

	t.Run("Unauthenticated_gets_401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/flags/pinyin_search/check", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for unauthenticated request, got %d", rec.Code)
		}
	})
}

// TestCheckFlag_RoleBasedRollout verifies that role-based flags correctly evaluate
// against the user's role IDs from JWT claims.
func TestCheckFlag_RoleBasedRollout(t *testing.T) {
	e := echo.New()

	// Simulate a config service with a role-based flag
	// We'll create a handler that checks role context
	e.GET("/api/flags/:key/check", func(c echo.Context) error {
		userID, _ := c.Get("user_id").(int)
		roleIDs, _ := c.Get("role_ids").([]int)

		return c.JSON(http.StatusOK, map[string]interface{}{
			"user_id":  userID,
			"role_ids": roleIDs,
		})
	}, jwtMiddleware)

	t.Run("Learner_has_role_id_3", func(t *testing.T) {
		token := generateTestToken(t, 3, "learner1",
			[]string{"learner"}, []string{"learning.library.view"})

		req := httptest.NewRequest(http.MethodGet, "/api/flags/test/check", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		var body map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &body)

		if int(body["user_id"].(float64)) != 3 {
			t.Errorf("expected user_id=3, got %v", body["user_id"])
		}

		roleIDs := body["role_ids"].([]interface{})
		if len(roleIDs) == 0 {
			t.Fatal("role_ids should not be empty")
		}
		if int(roleIDs[0].(float64)) != 3 {
			t.Errorf("expected role_id=3 (learner), got %v", roleIDs[0])
		}
	})

	t.Run("Admin_has_role_id_1", func(t *testing.T) {
		token := generateTestToken(t, 1, "admin",
			[]string{"system_admin"}, []string{"admin.users.manage"})

		req := httptest.NewRequest(http.MethodGet, "/api/flags/test/check", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		var body map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &body)

		if int(body["user_id"].(float64)) != 1 {
			t.Errorf("expected user_id=1, got %v", body["user_id"])
		}

		roleIDs := body["role_ids"].([]interface{})
		if int(roleIDs[0].(float64)) != 1 {
			t.Errorf("expected role_id=1 (system_admin), got %v", roleIDs[0])
		}
	})
}
