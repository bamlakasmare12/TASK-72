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
	if u := os.Getenv("TEST_API_URL"); u != "" {
		return u
	}
	return "http://localhost:8081"
}

// TestMain registers the two smoke users before any test runs so individual
// tests can assume both accounts exist.
func TestMain(m *testing.M) {
	if err := setupUsers(); err != nil {
		fmt.Fprintf(os.Stderr, "smoke setup failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// setupUsers registers smokeadmin and smokelearner, tolerating conflicts so
// the suite is idempotent against a reused database.
//
// The admin account uses "learner" as the requested role because system_admin
// is not a valid self-registration role; the backend auto-assigns system_admin
// to the very first registered user regardless of the requested role.
func setupUsers() error {
	users := []struct {
		username, email, password, displayName, role string
	}{
		{"smokeadmin", "smokeadmin@smoke.local", "SmokeAdmin@2024!", "Smoke Admin", "learner"},
		{"smokelearner", "smokelearner@smoke.local", "SmokeLearner@2024!", "Smoke Learner", "learner"},
	}

	for _, u := range users {
		body, _ := json.Marshal(map[string]string{
			"username":     u.username,
			"email":        u.email,
			"password":     u.password,
			"display_name": u.displayName,
			"role":         u.role,
		})
		req, _ := http.NewRequest(http.MethodPost, apiURL()+"/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-App-Version", appVersion)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("register %s: %w", u.username, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK &&
			resp.StatusCode != http.StatusCreated &&
			resp.StatusCode != http.StatusConflict {
			return fmt.Errorf("register %s: unexpected status %d", u.username, resp.StatusCode)
		}
	}
	return nil
}

// apiRequest sends an HTTP request with the standard app-version header and an
// optional bearer token. It fatally fails the test on network error.
func apiRequest(t *testing.T, method, path string, body any, token string) *http.Response {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, apiURL()+path, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
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
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// decodeJSON decodes the response body into dst and closes it.
func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

// loginUser logs in as username/password and returns the bearer token.
func loginUser(t *testing.T, username, password string) string {
	t.Helper()
	resp := apiRequest(t, http.MethodPost, "/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, "")
	var result map[string]any
	decodeJSON(t, resp, &result)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login %q: status %d, body: %v", username, resp.StatusCode, result)
	}
	token, _ := result["token"].(string)
	if token == "" {
		t.Fatalf("login %q: no token in response: %v", username, result)
	}
	return token
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSmoke_HealthCheck(t *testing.T) {
	resp := apiRequest(t, http.MethodGet, "/api/health", nil, "")
	var result map[string]any
	decodeJSON(t, resp, &result)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health check: status %d, body: %v", resp.StatusCode, result)
	}
	if status, _ := result["status"].(string); status != "ok" {
		t.Fatalf("health status: want %q, got %q", "ok", status)
	}
}

func TestSmoke_AdminLogin(t *testing.T) {
	token := loginUser(t, "smokeadmin", "SmokeAdmin@2024!")
	t.Logf("admin login OK, token length=%d", len(token))
}

func TestSmoke_AuthMe(t *testing.T) {
	token := loginUser(t, "smokeadmin", "SmokeAdmin@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/auth/me", nil, token)
	var result map[string]any
	decodeJSON(t, resp, &result)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/auth/me: status %d, body: %v", resp.StatusCode, result)
	}
	if _, ok := result["user_id"]; !ok {
		t.Fatalf("GET /api/auth/me: missing user_id, got: %v", result)
	}
}

func TestSmoke_Search(t *testing.T) {
	token := loginUser(t, "smokeadmin", "SmokeAdmin@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/search?q=test", nil, token)
	var result map[string]any
	decodeJSON(t, resp, &result)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/search: status %d, body: %v", resp.StatusCode, result)
	}
	if _, ok := result["results"]; !ok {
		t.Fatalf("GET /api/search: missing results key, got: %v", result)
	}
}

func TestSmoke_LearningPaths(t *testing.T) {
	token := loginUser(t, "smokeadmin", "SmokeAdmin@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/learning/paths", nil, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /api/learning/paths: status %d, body: %s", resp.StatusCode, body)
	}
}

// TestSmoke_RBAC_LearnerCannotAccessSettlements verifies that a learner gets
// 404 (feature invisible) on a procurement-only endpoint, not 403.
func TestSmoke_RBAC_LearnerCannotAccessSettlements(t *testing.T) {
	token := loginUser(t, "smokelearner", "SmokeLearner@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/procurement/settlements", nil, token)
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("RBAC settlements: want 404, got %d", resp.StatusCode)
	}
}

// TestSmoke_RBAC_LearnerCanAccessSearch verifies that learners can use search.
func TestSmoke_RBAC_LearnerCanAccessSearch(t *testing.T) {
	token := loginUser(t, "smokelearner", "SmokeLearner@2024!")

	resp := apiRequest(t, http.MethodGet, "/api/search?q=test", nil, token)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("RBAC search: want 200, got %d", resp.StatusCode)
	}
}

// TestSmoke_VersionGate_MissingHeader verifies that requests without
// X-App-Version are rejected with 426 Upgrade Required.
func TestSmoke_VersionGate_MissingHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, apiURL()+"/api/auth/register",
		bytes.NewBufferString(`{"username":"x","email":"x@x.com","password":"X@1234567","display_name":"X","role":"learner"}`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Intentionally omit X-App-Version

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("version gate: want 426, got %d", resp.StatusCode)
	}
}
