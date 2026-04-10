package handlers

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/repository"
	"wlpr-portal/internal/services"

	"github.com/labstack/echo/v4"
)

type LearningHandler struct {
	learningService *services.LearningService
	learningRepo    *repository.LearningRepository
}

func NewLearningHandler(learningService *services.LearningService, learningRepo ...*repository.LearningRepository) *LearningHandler {
	h := &LearningHandler{learningService: learningService}
	if len(learningRepo) > 0 {
		h.learningRepo = learningRepo[0]
	}
	return h
}

// GET /api/learning/paths
func (h *LearningHandler) GetPaths(c echo.Context) error {
	paths, err := h.learningService.GetAllPaths(c.Request().Context())
	if err != nil {
		log.Printf("[learning] get paths error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve learning paths")
	}
	return c.JSON(http.StatusOK, paths)
}

// GET /api/learning/paths/:id
func (h *LearningHandler) GetPath(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid path ID")
	}

	path, err := h.learningService.GetPathDetail(c.Request().Context(), id)
	if err != nil || path == nil {
		return echo.NewHTTPError(http.StatusNotFound, "learning path not found")
	}
	return c.JSON(http.StatusOK, path)
}

// POST /api/learning/enroll
func (h *LearningHandler) Enroll(c echo.Context) error {
	userID := c.Get("user_id").(int)
	var body struct {
		PathID int `json:"path_id"`
	}
	if err := c.Bind(&body); err != nil || body.PathID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "path_id is required")
	}

	enrollment, err := h.learningService.Enroll(c.Request().Context(), userID, body.PathID)
	if err != nil {
		log.Printf("[learning] enroll error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "enrollment failed")
	}
	return c.JSON(http.StatusCreated, enrollment)
}

// DELETE /api/learning/enroll/:path_id
func (h *LearningHandler) DropEnrollment(c echo.Context) error {
	userID := c.Get("user_id").(int)
	pathID, err := strconv.Atoi(c.Param("path_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid path ID")
	}

	if err := h.learningService.DropEnrollment(c.Request().Context(), userID, pathID); err != nil {
		log.Printf("[learning] drop enrollment error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to drop enrollment")
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "enrollment dropped"})
}

// GET /api/learning/enrollments
func (h *LearningHandler) GetEnrollments(c echo.Context) error {
	userID := c.Get("user_id").(int)
	enrollments, err := h.learningService.GetEnrollments(c.Request().Context(), userID)
	if err != nil {
		log.Printf("[learning] get enrollments error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve enrollments")
	}
	return c.JSON(http.StatusOK, enrollments)
}

// GET /api/learning/enrollments/:path_id
func (h *LearningHandler) GetEnrollmentDetail(c echo.Context) error {
	userID := c.Get("user_id").(int)
	pathID, err := strconv.Atoi(c.Param("path_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid path ID")
	}

	detail, err := h.learningService.GetEnrollmentDetail(c.Request().Context(), userID, pathID)
	if err != nil {
		log.Printf("[learning] get enrollment detail error: %v", err)
		return echo.NewHTTPError(http.StatusNotFound, "enrollment not found")
	}
	return c.JSON(http.StatusOK, detail)
}

// PUT /api/learning/progress
func (h *LearningHandler) UpdateProgress(c echo.Context) error {
	userID := c.Get("user_id").(int)
	var req models.UpdateProgressRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.ResourceID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "resource_id is required")
	}
	if req.Status == "" {
		req.Status = "in_progress"
	}

	progress, err := h.learningService.UpdateProgress(c.Request().Context(), userID, req)
	if err != nil {
		log.Printf("[learning] update progress error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update progress")
	}
	return c.JSON(http.StatusOK, progress)
}

// GET /api/learning/progress?path_id=1
func (h *LearningHandler) GetProgress(c echo.Context) error {
	userID := c.Get("user_id").(int)
	var pathID *int
	if pid := c.QueryParam("path_id"); pid != "" {
		if id, err := strconv.Atoi(pid); err == nil {
			pathID = &id
		}
	}

	progress, err := h.learningService.GetProgress(c.Request().Context(), userID, pathID)
	if err != nil {
		log.Printf("[learning] get progress error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve progress")
	}
	return c.JSON(http.StatusOK, progress)
}

// GET /api/learning/export
// Streams a CSV download of the authenticated user's learning records.
func (h *LearningHandler) ExportCSV(c echo.Context) error {
	userID := c.Get("user_id").(int)

	records, err := h.learningService.GetLearningRecords(c.Request().Context(), userID)
	if err != nil {
		log.Printf("[learning] export CSV error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to export learning records")
	}

	c.Response().Header().Set("Content-Type", "text/csv")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=learning_records.csv")
	c.Response().WriteHeader(http.StatusOK)

	writer := csv.NewWriter(c.Response().Writer)

	// Write header
	_ = writer.Write([]string{
		"Username", "Resource Title", "Learning Path", "Status",
		"Progress %", "Time Spent (mins)", "Started At", "Completed At",
	})

	// Stream rows
	for _, r := range records {
		_ = writer.Write([]string{
			r.Username,
			r.ResourceTitle,
			r.PathTitle,
			r.Status,
			fmt.Sprintf("%d", r.ProgressPct),
			fmt.Sprintf("%d", r.TimeSpentMin),
			r.StartedAt,
			r.CompletedAt,
		})
	}

	writer.Flush()
	return nil
}

// GET /api/learning/recommendations?limit=20
func (h *LearningHandler) GetRecommendations(c echo.Context) error {
	userID := c.Get("user_id").(int)
	limit := 20
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil && l > 0 {
		limit = l
	}

	recs, err := h.learningService.GetRecommendations(c.Request().Context(), userID, limit)
	if err != nil {
		log.Printf("[learning] get recommendations error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve recommendations")
	}
	return c.JSON(http.StatusOK, recs)
}

// POST /api/admin/learning/paths
func (h *LearningHandler) CreatePath(c echo.Context) error {
	var req models.CreateLearningPathRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Title == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "title is required")
	}

	userID := c.Get("user_id").(int)
	lp := models.LearningPath{
		Title:           req.Title,
		Description:     req.Description,
		CategoryID:      req.CategoryID,
		TargetJobFamily: req.TargetJobFamily,
		RequiredCount:   req.RequiredCount,
		ElectiveMin:     req.ElectiveMin,
		EstimatedHours:  req.EstimatedHours,
		Difficulty:      req.Difficulty,
	}

	created, err := h.learningRepo.CreateLearningPath(c.Request().Context(), lp, userID)
	if err != nil {
		log.Printf("[learning] create path error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create learning path")
	}
	return c.JSON(http.StatusCreated, created)
}

// POST /api/admin/learning/paths/items
func (h *LearningHandler) AddPathItem(c echo.Context) error {
	var req models.AddPathItemRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.PathID == 0 || req.ResourceID == 0 || req.ItemType == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "path_id, resource_id, and item_type are required")
	}
	if req.ItemType != "required" && req.ItemType != "elective" {
		return echo.NewHTTPError(http.StatusBadRequest, "item_type must be 'required' or 'elective'")
	}

	if err := h.learningRepo.AddPathItem(c.Request().Context(), req.PathID, req.ResourceID, req.ItemType, req.SortOrder); err != nil {
		log.Printf("[learning] add path item error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add path item")
	}
	return c.JSON(http.StatusCreated, map[string]string{"message": "path item added"})
}
