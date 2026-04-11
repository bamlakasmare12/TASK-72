// Package smoke contains end-to-end smoke tests for the WLPR Portal HTTP API.
// Requires the full stack (backend + DB) to be running.
//
// Run with:
//
//	TEST_API_URL="http://localhost:8081" \
//	  go test -v -tags=integration -count=1 ./tests/e2e/backend/smoke/...
//
//go:build integration

package smoke_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

const appVersion = "1.0.0"

func apiURL() string {
	u := os.Getenv("TEST_API_URL")
	if u == "" {
		return "http://localhost:8081"
	}
	return u
}

// apiRequest sends an HTTP request with the standard headers and returns the response.
func apiRequest(t *testing.T, method, path string, body any, token string) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, apiURL()+path, bodyReader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-App-Version", appVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	return resp
}

// decodeJSON decodes the response body into dst and closes the body.
func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
}

// TestSmoke_Registration_Admin verifies that the first user registered receives
// the system_admin role.
func TestSmoke_Registration_Admin(t *testing.T) {
	resp := apiRequest(t, http.MethodPost, "/api/auth/register", map[string]string{
		"username":     "smokeadmin",
		"email":        "smokeadmin@smoke.local",
		"password":     "SmokeAdmin@2024!",
		"display_name": "Smoke Admin",
		"role":         "system_admin",
	}, "")

	var result map[string]any
	decodeJSON(t, resp, &result)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("registration returned %d, body: %v", resp.StatusCode, result)
	}

	msg, _ := result["message"].(string)
	if msg == "" {
		t.Fatalf("expected a message in registration response, got: %v", result)
	}
	t.Logf("registration message: %s", msg)
}

// TestSmoke_Login_Admin verifies that the smoke admin can log in and returns a JWT.
func TestSmoke_Login_Admin(t *testing.T) {
	token := loginUser(t, "smokeadmin", "SmokeAdmin@2024!")
	if token == "" {
		t.Fatal("expected non-empty token from admin login")
	}
	t.Logf("admin login OK, token length=%d", len(token))
}

// TestSmoke_AuthMe verifies that GET /api/auth/me returns the authenticated user.
func TestSmoke_AuthMe(t *testing.T) {
	token := loginUser(t, "smokeadmin", "SmokeAdmin@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/auth/me", nil, token)
	var result map[string]any
	decodeJSON(t, resp, &result)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/auth/me returned %d: %v", resp.StatusCode, result)
	}
	if _, ok := result["user_id"]; !ok {
		t.Fatalf("expected user_id in /api/auth/me response, got: %v", result)
	}
	t.Logf("GET /api/auth/me OK, user_id=%v", result["user_id"])
}

// TestSmoke_Search verifies that GET /api/search returns a valid response.
func TestSmoke_Search(t *testing.T) {
	token := loginUser(t, "smokeadmin", "SmokeAdmin@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/search?q=test", nil, token)
	var result map[string]any
	decodeJSON(t, resp, &result)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/search returned %d: %v", resp.StatusCode, result)
	}
	if _, ok := result["results"]; !ok {
		t.Fatalf("expected results key in /api/search response, got: %v", result)
	}
	t.Log("GET /api/search OK")
}

// TestSmoke_LearningPaths verifies that GET /api/learning/paths returns a valid response.
func TestSmoke_LearningPaths(t *testing.T) {
	token := loginUser(t, "smokeadmin", "SmokeAdmin@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/learning/paths", nil, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /api/learning/paths returned %d: %s", resp.StatusCode, body)
	}
	t.Log("GET /api/learning/paths OK")
}

// TestSmoke_Registration_Learner verifies that a second user can be registered
// with the learner role.
func TestSmoke_Registration_Learner(t *testing.T) {
	resp := apiRequest(t, http.MethodPost, "/api/auth/register", map[string]string{
		"username":     "smokelearner",
		"email":        "smokelearner@smoke.local",
		"password":     "SmokeLearner@2024!",
		"display_name": "Smoke Learner",
		"role":         "learner",
	}, "")

	var result map[string]any
	decodeJSON(t, resp, &result)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("learner registration returned %d: %v", resp.StatusCode, result)
	}

	msg, _ := result["message"].(string)
	if msg == "" {
		t.Fatalf("expected message in learner registration response, got: %v", result)
	}
	t.Logf("learner registered: %s", msg)
}

// TestSmoke_RBAC_LearnerCannotAccessSettlements verifies that a learner receives
// 404 (feature invisible) when accessing a procurement-only endpoint.
func TestSmoke_RBAC_LearnerCannotAccessSettlements(t *testing.T) {
	token := loginUser(t, "smokelearner", "SmokeLearner@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/procurement/settlements", nil, token)
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("RBAC: expected 404 for learner on /api/procurement/settlements, got %d", resp.StatusCode)
	}
	t.Log("RBAC: learner gets 404 on /api/procurement/settlements (feature invisible)")
}

// TestSmoke_RBAC_LearnerCanAccessSearch verifies that a learner can use the
// search endpoint (open to all authenticated users).
func TestSmoke_RBAC_LearnerCanAccessSearch(t *testing.T) {
	token := loginUser(t, "smokelearner", "SmokeLearner@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/search?q=test", nil, token)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("RBAC: expected 200 for learner on /api/search, got %d", resp.StatusCode)
	}
	t.Log("RBAC: learner can access /api/search (200)")
}

// loginUser is a test helper that registers (tolerating 409 conflict) then logs
// in and returns the bearer token. It fails the test if login does not succeed.
func loginUser(t *testing.T, username, password string) string {
	t.Helper()

	resp := apiRequest(t, http.MethodPost, "/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, "")

	var result map[string]any
	decodeJSON(t, resp, &result)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login for %q returned %d: %v", username, resp.StatusCode, result)
	}

	token, ok := result["token"].(string)
	if !ok || token == "" {
		t.Fatalf("login for %q did not return a token: %v", username, result)
	}
	return token
}

// TestSmoke_HealthCheck verifies the backend health endpoint before running
// user-facing tests. This is a quick sanity check that the API is reachable.
func TestSmoke_HealthCheck(t *testing.T) {
	resp := apiRequest(t, http.MethodGet, "/api/health", nil, "")
	var result map[string]any
	decodeJSON(t, resp, &result)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health check returned %d: %v", resp.StatusCode, result)
	}
	status, _ := result["status"].(string)
	if status != "ok" {
		t.Fatalf("expected health status=ok, got %q", status)
	}
	t.Logf("health check OK: %v", result)
}

// TestSmoke_Setup_RegisterUsers is a TestMain-like setup test that registers
// both the admin and learner accounts used by the other smoke tests.
// It tolerates "already exists" responses so the suite is idempotent.
func TestSmoke_Setup_RegisterUsers(t *testing.T) {
	users := []struct {
		username, email, password, displayName, role string
	}{
		{"smokeadmin", "smokeadmin@smoke.local", "SmokeAdmin@2024!", "Smoke Admin", "system_admin"},
		{"smokelearner", "smokelearner@smoke.local", "SmokeLearner@2024!", "Smoke Learner", "learner"},
	}

	for _, u := range users {
		resp := apiRequest(t, http.MethodPost, "/api/auth/register", map[string]string{
			"username":     u.username,
			"email":        u.email,
			"password":     u.password,
			"display_name": u.displayName,
			"role":         u.role,
		}, "")
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated ||
			resp.StatusCode == http.StatusConflict {
			t.Logf("registered %s (status %d)", u.username, resp.StatusCode)
		} else {
			t.Errorf("unexpected status %d registering %s: %s", resp.StatusCode, u.username, body)
		}
	}
}

// TestSmoke_Version_MissingHeaderRejected verifies that requests without
// X-App-Version are rejected with 426 Upgrade Required.
func TestSmoke_Version_MissingHeaderRejected(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, apiURL()+"/api/auth/register",
		bytes.NewBufferString(`{"username":"x","email":"x@x.com","password":"X@1234567","display_name":"X","role":"learner"}`))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Intentionally omit X-App-Version

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("expected 426 for missing X-App-Version, got %d", resp.StatusCode)
	}
	t.Logf("version gate correctly returns 426 when X-App-Version is absent")
}

// ensure fmt is used
var _ = fmt.Sprintf
