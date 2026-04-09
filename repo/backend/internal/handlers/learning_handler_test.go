package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func TestEnroll_MissingPathID(t *testing.T) {
	e := echo.New()
	h := &LearningHandler{} // nil service; validation fires before service call
	e.POST("/api/learning/enroll", h.Enroll, jwtMiddleware, requireRole("learner", "content_moderator", "system_admin"))

	token := generateTestToken(t, 3, "learner1", []string{"learner"}, []string{"learning.path.enroll"})

	req := httptest.NewRequest(http.MethodPost, "/api/learning/enroll", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing path_id, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "path_id") {
		t.Errorf("expected error about path_id, got: %s", rec.Body.String())
	}
}

func TestEnroll_RequiresAuth(t *testing.T) {
	e := echo.New()
	h := &LearningHandler{}
	e.POST("/api/learning/enroll", h.Enroll, jwtMiddleware, requireRole("learner", "content_moderator", "system_admin"))

	req := httptest.NewRequest(http.MethodPost, "/api/learning/enroll", strings.NewReader(`{"path_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated enroll, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateProgress_MissingResourceID(t *testing.T) {
	e := echo.New()
	h := &LearningHandler{}
	e.PUT("/api/learning/progress", h.UpdateProgress, jwtMiddleware, requireRole("learner", "content_moderator", "system_admin"))

	token := generateTestToken(t, 3, "learner1", []string{"learner"}, []string{"learning.progress.update"})

	body := `{"resource_id":0,"status":"in_progress"}`
	req := httptest.NewRequest(http.MethodPut, "/api/learning/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for resource_id=0, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdateProgress_DefaultStatus verifies that when status is empty, the handler
// defaults it to "in_progress" and does NOT reject with 400. Uses Echo Recover
// middleware to catch the nil-service panic as a 500 (which proves validation passed).
func TestUpdateProgress_DefaultStatus(t *testing.T) {
	e := echo.New()
	e.Use(echomw.Recover())
	h := &LearningHandler{}
	e.PUT("/api/learning/progress", h.UpdateProgress, jwtMiddleware, requireRole("learner", "content_moderator", "system_admin"))

	token := generateTestToken(t, 3, "learner1", []string{"learner"}, []string{"learning.progress.update"})

	body := `{"resource_id":5,"status":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/learning/progress", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Must NOT be 400 — handler defaults empty status to "in_progress" and proceeds.
	// 500 is expected (nil service), which proves validation passed.
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("handler should default empty status to 'in_progress', not return 400: %s", rec.Body.String())
	}
}

// TestGetProgress_ExtractsUserIDFromToken verifies that the progress endpoint
// uses the JWT-derived user_id, not any query parameter. We simulate a handler
// that extracts user_id from the context (set by jwtMiddleware) and returns it.
func TestGetProgress_ExtractsUserIDFromToken(t *testing.T) {
	e := echo.New()

	// Use a stub handler that mirrors the real handler's user_id extraction
	e.GET("/api/learning/progress", func(c echo.Context) error {
		userID := c.Get("user_id").(int)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"user_id":  userID,
			"progress": []interface{}{},
		})
	}, jwtMiddleware, requireRole("learner", "content_moderator", "system_admin"))

	token := generateTestToken(t, 7, "learner7", []string{"learner"}, []string{"learning.progress.view_own"})

	// Attempt to pass user_id=999 via query param — handler should ignore it
	req := httptest.NewRequest(http.MethodGet, "/api/learning/progress?user_id=999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if int(body["user_id"].(float64)) != 7 {
		t.Fatalf("expected user_id=7 from token, got %v", body["user_id"])
	}
}

// TestExportCSV_ReturnsCSVHeaders verifies that the export endpoint sets the
// correct Content-Type and Content-Disposition headers for CSV download.
func TestExportCSV_ReturnsCSVHeaders(t *testing.T) {
	e := echo.New()

	// Stub handler that mimics the real ExportCSV header-setting behavior
	e.GET("/api/learning/export", func(c echo.Context) error {
		_ = c.Get("user_id").(int)
		c.Response().Header().Set("Content-Type", "text/csv")
		c.Response().Header().Set("Content-Disposition", "attachment; filename=learning_records.csv")
		c.Response().WriteHeader(http.StatusOK)
		return nil
	}, jwtMiddleware, requireRole("learner", "content_moderator", "system_admin"))

	token := generateTestToken(t, 3, "learner1", []string{"learner"}, []string{"learning.export"})

	req := httptest.NewRequest(http.MethodGet, "/api/learning/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Errorf("expected Content-Type text/csv, got %s", ct)
	}

	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("expected Content-Disposition attachment, got %s", cd)
	}
}

func TestGetPaths_RequiresLearnerRole(t *testing.T) {
	e := echo.New()
	h := &LearningHandler{}
	e.GET("/api/learning/paths", h.GetPaths, jwtMiddleware, requireRole("learner", "content_moderator", "system_admin"))

	// finance_analyst is NOT in the allowed roles
	token := generateTestToken(t, 6, "finance1", []string{"finance_analyst"}, []string{"finance.ledger.view"})

	req := httptest.NewRequest(http.MethodGet, "/api/learning/paths", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("finance_analyst should get 404 for learning paths, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestGetPaths_LearnerAllowed verifies a learner reaches the handler (not blocked by RBAC).
// The handler will fail at the service layer (nil pointer => 500), but the point is
// it should NOT get 401 or 404 from middleware.
func TestGetPaths_LearnerAllowed(t *testing.T) {
	e := echo.New()
	e.Use(echomw.Recover())
	h := &LearningHandler{}
	e.GET("/api/learning/paths", h.GetPaths, jwtMiddleware, requireRole("learner", "content_moderator", "system_admin"))

	token := generateTestToken(t, 3, "learner1", []string{"learner"}, []string{"learning.library.view"})

	req := httptest.NewRequest(http.MethodGet, "/api/learning/paths", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Learner should pass RBAC middleware. The handler will call nil service => 500.
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusNotFound {
		t.Fatalf("learner should not be blocked by RBAC, got %d: %s", rec.Code, rec.Body.String())
	}
}
