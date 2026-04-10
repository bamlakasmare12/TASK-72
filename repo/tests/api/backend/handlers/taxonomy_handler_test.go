package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wlpr-portal/internal/handlers"

	"github.com/labstack/echo/v4"
)

func TestCreateTag_MissingName(t *testing.T) {
	e := echo.New()
	h := &handlers.TaxonomyHandler{} // nil repo; validation runs before repo call
	e.POST("/api/taxonomy/tags", h.CreateTag, jwtMiddleware, requireRole("content_moderator", "system_admin"))

	token := generateTestToken(t, 8, "moderator1", []string{"content_moderator"}, []string{"taxonomy.tags.manage"})

	body := `{"name":"","tag_type":"skill"}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing tag name, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateTag_MissingType(t *testing.T) {
	e := echo.New()
	h := &handlers.TaxonomyHandler{}
	e.POST("/api/taxonomy/tags", h.CreateTag, jwtMiddleware, requireRole("content_moderator", "system_admin"))

	token := generateTestToken(t, 8, "moderator1", []string{"content_moderator"}, []string{"taxonomy.tags.manage"})

	body := `{"name":"Go Programming","tag_type":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing tag_type, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateTag_RequiresModeratorRole(t *testing.T) {
	e := echo.New()
	h := &handlers.TaxonomyHandler{}
	e.POST("/api/taxonomy/tags", h.CreateTag, jwtMiddleware, requireRole("content_moderator", "system_admin"))

	// learner is not in the allowed roles for tag creation
	token := generateTestToken(t, 3, "learner1", []string{"learner"}, []string{"learning.library.view"})

	body := `{"name":"New Tag","tag_type":"skill"}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/tags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("learner should get 404 for tag creation, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateSynonym_MissingTerm(t *testing.T) {
	e := echo.New()
	h := &handlers.TaxonomyHandler{}
	e.POST("/api/taxonomy/synonyms", h.CreateSynonym, jwtMiddleware, requireRole("content_moderator", "system_admin"))

	token := generateTestToken(t, 8, "moderator1", []string{"content_moderator"}, []string{"taxonomy.synonyms.manage"})

	body := `{"term":"","canonical_tag_id":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/synonyms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing synonym term, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateSynonym_MissingCanonicalID(t *testing.T) {
	e := echo.New()
	h := &handlers.TaxonomyHandler{}
	e.POST("/api/taxonomy/synonyms", h.CreateSynonym, jwtMiddleware, requireRole("content_moderator", "system_admin"))

	token := generateTestToken(t, 8, "moderator1", []string{"content_moderator"}, []string{"taxonomy.synonyms.manage"})

	body := `{"term":"golang","canonical_tag_id":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/synonyms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing canonical_tag_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateSynonym_RequiresModeratorRole(t *testing.T) {
	e := echo.New()
	h := &handlers.TaxonomyHandler{}
	e.POST("/api/taxonomy/synonyms", h.CreateSynonym, jwtMiddleware, requireRole("content_moderator", "system_admin"))

	// finance_analyst is not in the allowed roles for synonym creation
	token := generateTestToken(t, 6, "finance1", []string{"finance_analyst"}, []string{"finance.ledger.view"})

	body := `{"term":"golang","canonical_tag_id":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/synonyms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("finance_analyst should get 404 for synonym creation, got %d: %s", rec.Code, rec.Body.String())
	}
}
