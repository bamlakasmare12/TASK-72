package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestApproveReviewItem_MissingReviewItemID(t *testing.T) {
	e := echo.New()
	h := &TaxonomyHandler{}
	e.POST("/api/taxonomy/review-queue/approve", h.ApproveReviewItem,
		jwtMiddleware, requireRole("content_moderator", "system_admin"))

	token := generateTestToken(t, 8, "moderator1",
		[]string{"content_moderator"}, []string{"taxonomy.tags.manage"})

	body := `{"review_item_id":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/review-queue/approve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing review_item_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRejectReviewItem_MissingDecisionNotes(t *testing.T) {
	e := echo.New()
	h := &TaxonomyHandler{}
	e.POST("/api/taxonomy/review-queue/reject", h.RejectReviewItem,
		jwtMiddleware, requireRole("content_moderator", "system_admin"))

	token := generateTestToken(t, 8, "moderator1",
		[]string{"content_moderator"}, []string{"taxonomy.tags.manage"})

	body := `{"review_item_id":1,"decision_notes":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/review-queue/reject", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing decision_notes on rejection, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApproveReviewItem_RequiresModeratorRole(t *testing.T) {
	e := echo.New()
	h := &TaxonomyHandler{}
	e.POST("/api/taxonomy/review-queue/approve", h.ApproveReviewItem,
		jwtMiddleware, requireRole("content_moderator", "system_admin"))

	// learner should not be able to approve
	token := generateTestToken(t, 3, "learner1",
		[]string{"learner"}, []string{"learning.library.view"})

	body := `{"review_item_id":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/review-queue/approve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("learner should get 404 for review queue actions, got %d", rec.Code)
	}
}

func TestRejectReviewItem_RequiresModeratorRole(t *testing.T) {
	e := echo.New()
	h := &TaxonomyHandler{}
	e.POST("/api/taxonomy/review-queue/reject", h.RejectReviewItem,
		jwtMiddleware, requireRole("content_moderator", "system_admin"))

	// procurement_specialist should not be able to reject taxonomy items
	token := generateTestToken(t, 4, "procuser",
		[]string{"procurement_specialist"}, []string{"procurement.orders.manage"})

	body := `{"review_item_id":1,"decision_notes":"bad content"}`
	req := httptest.NewRequest(http.MethodPost, "/api/taxonomy/review-queue/reject", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("procurement_specialist should get 404 for review queue actions, got %d", rec.Code)
	}
}

func TestGetReviewQueueAudit_RequiresModeratorRole(t *testing.T) {
	e := echo.New()
	h := &TaxonomyHandler{}
	e.GET("/api/taxonomy/review-queue/audit", h.GetReviewQueueAudit,
		jwtMiddleware, requireRole("content_moderator", "system_admin"))

	// finance analyst should not see taxonomy audit trail
	token := generateTestToken(t, 6, "finance1",
		[]string{"finance_analyst"}, []string{"finance.reconciliation.view"})

	req := httptest.NewRequest(http.MethodGet, "/api/taxonomy/review-queue/audit", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("finance_analyst should get 404 for review queue audit, got %d", rec.Code)
	}
}
