package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/repository"

	"github.com/labstack/echo/v4"
)

type TaxonomyHandler struct {
	repo *repository.TaxonomyRepository
}

func NewTaxonomyHandler(repo *repository.TaxonomyRepository) *TaxonomyHandler {
	return &TaxonomyHandler{repo: repo}
}

// GET /api/taxonomy/tags?type=skill
func (h *TaxonomyHandler) GetTags(c echo.Context) error {
	tagType := c.QueryParam("type")
	tags, err := h.repo.GetAllTags(c.Request().Context(), tagType)
	if err != nil {
		log.Printf("[taxonomy] get tags error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve tags")
	}
	return c.JSON(http.StatusOK, tags)
}

// GET /api/taxonomy/tags/hierarchy?type=job
func (h *TaxonomyHandler) GetHierarchy(c echo.Context) error {
	tagType := c.QueryParam("type")
	hierarchy, err := h.repo.GetTagHierarchy(c.Request().Context(), tagType)
	if err != nil {
		log.Printf("[taxonomy] get tag hierarchy error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve tag hierarchy")
	}
	return c.JSON(http.StatusOK, hierarchy)
}

// GET /api/taxonomy/tags/:id
func (h *TaxonomyHandler) GetTag(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tag ID")
	}

	tag, err := h.repo.GetTagByID(c.Request().Context(), id)
	if err != nil || tag == nil {
		return echo.NewHTTPError(http.StatusNotFound, "tag not found")
	}

	// Load children
	children, _ := h.repo.GetTagChildren(c.Request().Context(), id)
	tag.Children = children

	return c.JSON(http.StatusOK, tag)
}

// POST /api/taxonomy/tags
// Creates a tag with 'pending' status. It must be approved via the review queue.
func (h *TaxonomyHandler) CreateTag(c echo.Context) error {
	var req models.CreateTagRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Name) == "" || req.TagType == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name and tag_type are required")
	}

	userID := c.Get("user_id").(int)
	tag, err := h.repo.CreateTag(c.Request().Context(), req, userID)
	if err != nil {
		log.Printf("[taxonomy] create tag error: %v", err)
		return echo.NewHTTPError(http.StatusConflict, "tag already exists or conflicts with an existing tag")
	}
	return c.JSON(http.StatusCreated, tag)
}

// GET /api/taxonomy/synonyms/:tag_id
func (h *TaxonomyHandler) GetSynonyms(c echo.Context) error {
	tagID, err := strconv.Atoi(c.Param("tag_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tag ID")
	}

	synonyms, err := h.repo.GetSynonyms(c.Request().Context(), tagID)
	if err != nil {
		log.Printf("[taxonomy] get synonyms error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve synonyms")
	}
	return c.JSON(http.StatusOK, synonyms)
}

// POST /api/taxonomy/synonyms
// Creates a synonym with 'pending_review' status. It must be approved via the review queue.
func (h *TaxonomyHandler) CreateSynonym(c echo.Context) error {
	var req models.CreateSynonymRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Term) == "" || req.CanonicalTagID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "term and canonical_tag_id are required")
	}

	userID := c.Get("user_id").(int)
	synonym, err := h.repo.CreateSynonym(c.Request().Context(), req.Term, req.CanonicalTagID, userID)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "Synonym conflict") {
			log.Printf("[taxonomy] create synonym conflict: %v", err)
			return echo.NewHTTPError(http.StatusConflict, "synonym conflicts with an existing entry")
		}
		log.Printf("[taxonomy] create synonym error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create synonym")
	}
	return c.JSON(http.StatusCreated, synonym)
}

// GET /api/taxonomy/review-queue
// Returns pending review items for moderator approval.
func (h *TaxonomyHandler) GetReviewQueue(c echo.Context) error {
	queue, err := h.repo.GetReviewQueue(c.Request().Context())
	if err != nil {
		log.Printf("[taxonomy] get review queue error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve review queue")
	}
	return c.JSON(http.StatusOK, queue)
}

// GET /api/taxonomy/review-queue/audit
// Returns full review queue history (all statuses) for audit trail.
func (h *TaxonomyHandler) GetReviewQueueAudit(c echo.Context) error {
	queue, err := h.repo.GetReviewQueueAll(c.Request().Context())
	if err != nil {
		log.Printf("[taxonomy] get review queue audit error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve review audit trail")
	}
	return c.JSON(http.StatusOK, queue)
}

// POST /api/taxonomy/review-queue/approve
// Approves a pending review queue item, activating the associated tag or synonym.
func (h *TaxonomyHandler) ApproveReviewItem(c echo.Context) error {
	var req models.ReviewActionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.ReviewItemID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "review_item_id is required")
	}

	reviewerID := c.Get("user_id").(int)
	if err := h.repo.ApproveReviewItem(c.Request().Context(), req.ReviewItemID, reviewerID, req.DecisionNotes); err != nil {
		log.Printf("[taxonomy] approve review item error: %v", err)
		errMsg := err.Error()
		if strings.Contains(errMsg, "not pending") {
			return echo.NewHTTPError(http.StatusConflict, errMsg)
		}
		if strings.Contains(errMsg, "not found") {
			return echo.NewHTTPError(http.StatusNotFound, "review item not found")
		}
		if strings.Contains(errMsg, "Synonym conflict") {
			return echo.NewHTTPError(http.StatusConflict, "synonym conflicts with an existing active entry")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to approve review item")
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "review item approved"})
}

// POST /api/taxonomy/review-queue/reject
// Rejects a pending review queue item, marking the associated tag or synonym as rejected.
func (h *TaxonomyHandler) RejectReviewItem(c echo.Context) error {
	var req models.ReviewActionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.ReviewItemID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "review_item_id is required")
	}
	if strings.TrimSpace(req.DecisionNotes) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "decision_notes are required for rejection")
	}

	reviewerID := c.Get("user_id").(int)
	if err := h.repo.RejectReviewItem(c.Request().Context(), req.ReviewItemID, reviewerID, req.DecisionNotes); err != nil {
		log.Printf("[taxonomy] reject review item error: %v", err)
		errMsg := err.Error()
		if strings.Contains(errMsg, "not pending") {
			return echo.NewHTTPError(http.StatusConflict, errMsg)
		}
		if strings.Contains(errMsg, "not found") {
			return echo.NewHTTPError(http.StatusNotFound, "review item not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to reject review item")
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "review item rejected"})
}
