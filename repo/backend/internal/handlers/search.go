package handlers

import (
	"log"
	"net/http"
	"strconv"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/services"

	"github.com/labstack/echo/v4"
)

type SearchHandler struct {
	searchService *services.SearchService
}

func NewSearchHandler(searchService *services.SearchService) *SearchHandler {
	return &SearchHandler{searchService: searchService}
}

// GET /api/search?q=...&categories=1,2&tags=3,4&date_from=2024-01-01&date_to=2024-12-31&difficulty=beginner&type=course&sort_by=relevance&page=1&page_size=20
func (h *SearchHandler) Search(c echo.Context) error {
	req := models.SearchRequest{
		Query:      c.QueryParam("q"),
		DateFrom:   c.QueryParam("date_from"),
		DateTo:     c.QueryParam("date_to"),
		Difficulty: c.QueryParam("difficulty"),
		Type:       c.QueryParam("type"),
		SortBy:     c.QueryParam("sort_by"),
	}

	if p, err := strconv.Atoi(c.QueryParam("page")); err == nil {
		req.Page = p
	} else {
		req.Page = 1
	}
	if ps, err := strconv.Atoi(c.QueryParam("page_size")); err == nil {
		req.PageSize = ps
	} else {
		req.PageSize = 20
	}

	// Parse comma-separated category IDs
	req.Categories = parseIntList(c.QueryParam("categories"))
	req.Tags = parseIntList(c.QueryParam("tags"))

	// Resolve user context for feature flag evaluation
	userID, _ := c.Get("user_id").(int)
	userRoleIDs, _ := c.Get("role_ids").([]int)

	resp, err := h.searchService.Search(c.Request().Context(), req, userID, userRoleIDs)
	if err != nil {
		log.Printf("[search] search error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "search failed")
	}

	return c.JSON(http.StatusOK, resp)
}

// GET /api/resources/:id
func (h *SearchHandler) GetResource(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid resource ID")
	}

	resource, err := h.searchService.GetResource(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "resource not found")
	}

	// Record view event
	if userID, ok := c.Get("user_id").(int); ok {
		_ = h.searchService.RecordView(c.Request().Context(), userID, id)
	}

	return c.JSON(http.StatusOK, resource)
}

// GET /api/archives
func (h *SearchHandler) GetArchives(c echo.Context) error {
	archives, err := h.searchService.GetArchives(c.Request().Context())
	if err != nil {
		log.Printf("[search] get archives error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retrieve archives")
	}
	return c.JSON(http.StatusOK, archives)
}

func parseIntList(s string) []int {
	if s == "" {
		return nil
	}
	var result []int
	for _, part := range splitComma(s) {
		if id, err := strconv.Atoi(part); err == nil {
			result = append(result, id)
		}
	}
	return result
}

func splitComma(s string) []string {
	var parts []string
	current := ""
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
