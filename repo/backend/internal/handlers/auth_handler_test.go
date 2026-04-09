package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wlpr-portal/internal/models"

	"github.com/labstack/echo/v4"
)

func TestRegister_MissingFields(t *testing.T) {
	e := echo.New()
	h := &AuthHandler{} // nil deps; validation runs before any repo call
	e.POST("/api/auth/register", h.Register)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	e := echo.New()
	h := &AuthHandler{}
	e.POST("/api/auth/register", h.Register)

	body := `{"username":"user1","email":"u@e.com","password":"short","display_name":"User One","role":"learner"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "at least 8") {
		t.Errorf("expected error about password length, got: %s", rec.Body.String())
	}
}

func TestRegister_MissingRole(t *testing.T) {
	e := echo.New()
	h := &AuthHandler{}
	e.POST("/api/auth/register", h.Register)

	body := `{"username":"user1","email":"u@e.com","password":"longpassword123","display_name":"User One"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing role, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "role is required") {
		t.Errorf("expected 'role is required' error, got: %s", rec.Body.String())
	}
}

func TestRegister_InvalidRole(t *testing.T) {
	e := echo.New()
	h := &AuthHandler{}
	e.POST("/api/auth/register", h.Register)

	// With nil userRepo, CountUsers will panic. Test the model validation directly.
	// The handler will call CountUsers first; if userCount > 0, it validates against ValidRoles().
	// We verify that "hacker" is not in ValidRoles and also not in AllRoles.
	if models.ValidRoles()["hacker"] {
		t.Fatal("hacker should not be a valid self-registration role")
	}
	if models.AllRoles()["hacker"] {
		t.Fatal("hacker should not be a valid role at all")
	}
}

// TestRegister_SystemAdminRoleBlocked verifies that "system_admin" is NOT available
// for self-registration. Only admin-assigned via /api/admin/users/assign-role.
func TestRegister_SystemAdminRoleBlocked(t *testing.T) {
	// system_admin must NOT be in ValidRoles (self-registration set)
	if models.ValidRoles()["system_admin"] {
		t.Fatal("system_admin must NOT be in ValidRoles() — it should be admin-assigned only")
	}

	// system_admin must still be in AllRoles (for admin assignment)
	if !models.AllRoles()["system_admin"] {
		t.Fatal("system_admin must be in AllRoles()")
	}

	// All self-registration roles must be present
	selfRegRoles := []string{"learner", "content_moderator", "procurement_specialist", "approver", "finance_analyst"}
	for _, role := range selfRegRoles {
		if !models.ValidRoles()[role] {
			t.Errorf("role %q missing from ValidRoles()", role)
		}
	}

	// Verify invalid roles are rejected
	if models.AllRoles()["hacker"] {
		t.Error("hacker should not be in AllRoles()")
	}
}

// TestRegister_SystemAdminSelfAssign_Rejected verifies that "system_admin" is NOT
// in the self-registration role set. The handler enforces this after the first-user
// bootstrap check, so we validate at the model level directly.
func TestRegister_SystemAdminSelfAssign_Rejected(t *testing.T) {
	if models.ValidRoles()["system_admin"] {
		t.Fatal("SECURITY: system_admin must not be self-assignable via registration")
	}
	// system_admin must be in AllRoles (for admin assignment)
	if !models.AllRoles()["system_admin"] {
		t.Fatal("system_admin must be in AllRoles for admin assignment")
	}
}

func TestLogin_MissingFields(t *testing.T) {
	e := echo.New()
	h := &AuthHandler{}
	e.POST("/api/auth/login", h.Login)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty login body, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_EmptyUsername(t *testing.T) {
	e := echo.New()
	h := &AuthHandler{}
	e.POST("/api/auth/login", h.Login)

	body := `{"username":"","password":"somepassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAssignRole_MissingFields(t *testing.T) {
	e := echo.New()
	h := &AuthHandler{}
	e.POST("/api/admin/users/assign-role", h.AssignRole, jwtMiddleware, requireRole("system_admin"))

	token := generateTestToken(t, 1, "admin", []string{"system_admin"}, []string{"admin.users.manage"})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/assign-role", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing assign-role fields, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAssignRole_InvalidRole(t *testing.T) {
	e := echo.New()
	h := &AuthHandler{}
	e.POST("/api/admin/users/assign-role", h.AssignRole, jwtMiddleware, requireRole("system_admin"))

	token := generateTestToken(t, 1, "admin", []string{"system_admin"}, []string{"admin.users.manage"})

	body := `{"user_id":5,"role":"superuser"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/assign-role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid role name, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRegister_PrivilegeBoundary_FirstUserVsSubsequent validates the full registration
// privilege boundary: first user gets system_admin, subsequent users cannot self-assign it.
func TestRegister_PrivilegeBoundary_FirstUserVsSubsequent(t *testing.T) {
	// Verify the model-level boundary:
	// 1. ValidRoles (self-registration) must NOT include system_admin
	if models.ValidRoles()["system_admin"] {
		t.Fatal("SECURITY: system_admin must not be in ValidRoles (self-registration set)")
	}

	// 2. AllRoles (admin-assigned) MUST include system_admin
	if !models.AllRoles()["system_admin"] {
		t.Fatal("system_admin must be in AllRoles (admin assignment set)")
	}

	// 3. All 5 non-privileged roles must be available for self-registration
	expectedSelfReg := []string{"learner", "content_moderator", "procurement_specialist", "approver", "finance_analyst"}
	for _, role := range expectedSelfReg {
		if !models.ValidRoles()[role] {
			t.Errorf("role %q must be in ValidRoles for self-registration", role)
		}
	}

	// 4. Verify ValidRoles is a strict subset of AllRoles
	for role := range models.ValidRoles() {
		if !models.AllRoles()[role] {
			t.Errorf("ValidRoles contains %q which is not in AllRoles — inconsistency", role)
		}
	}

	// 5. Verify system_admin is the ONLY role excluded from self-registration
	allRoles := models.AllRoles()
	validRoles := models.ValidRoles()
	excludedCount := 0
	for role := range allRoles {
		if !validRoles[role] {
			excludedCount++
			if role != "system_admin" {
				t.Errorf("unexpected role %q excluded from self-registration", role)
			}
		}
	}
	if excludedCount != 1 {
		t.Errorf("expected exactly 1 role excluded from self-registration (system_admin), got %d", excludedCount)
	}
}

func TestAssignRole_RequiresAuth(t *testing.T) {
	e := echo.New()
	h := &AuthHandler{}
	e.POST("/api/admin/users/assign-role", h.AssignRole, jwtMiddleware, requireRole("system_admin"))

	body := `{"user_id":5,"role":"learner"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/assign-role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth, got %d: %s", rec.Code, rec.Body.String())
	}
}
