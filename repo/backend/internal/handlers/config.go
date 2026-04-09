package handlers

import (
	"log"
	"net/http"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/services"

	"github.com/labstack/echo/v4"
)

type ConfigHandler struct {
	configService *services.ConfigService
}

func NewConfigHandler(configService *services.ConfigService) *ConfigHandler {
	return &ConfigHandler{configService: configService}
}

// GET /api/config/all
func (h *ConfigHandler) GetAllConfigs(c echo.Context) error {
	configs := h.configService.GetAllConfigs()
	return c.JSON(http.StatusOK, configs)
}

// GET /api/config/:key
func (h *ConfigHandler) GetConfig(c echo.Context) error {
	key := c.Param("key")
	val, ok := h.configService.GetConfig(key)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "config not found")
	}
	return c.JSON(http.StatusOK, map[string]string{"key": key, "value": val})
}

// PUT /api/config/:key
func (h *ConfigHandler) UpdateConfig(c echo.Context) error {
	key := c.Param("key")
	var req models.ConfigUpdateRequest
	if err := c.Bind(&req); err != nil || req.Value == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "value is required")
	}

	userID := c.Get("user_id").(int)
	if err := h.configService.UpdateConfig(c.Request().Context(), key, req.Value, userID); err != nil {
		log.Printf("[config] update config error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update configuration")
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "config updated"})
}

// GET /api/config/flags
func (h *ConfigHandler) GetAllFlags(c echo.Context) error {
	flags := h.configService.GetAllFlags()
	return c.JSON(http.StatusOK, flags)
}

// GET /api/config/flags/:key
func (h *ConfigHandler) GetFlag(c echo.Context) error {
	key := c.Param("key")
	flags := h.configService.GetAllFlags()
	for _, f := range flags {
		if f.Key == key {
			return c.JSON(http.StatusOK, f)
		}
	}
	return echo.NewHTTPError(http.StatusNotFound, "flag not found")
}

// PUT /api/config/flags/:key
func (h *ConfigHandler) UpdateFlag(c echo.Context) error {
	key := c.Param("key")
	var req models.FeatureFlagUpdateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.configService.UpdateFeatureFlag(c.Request().Context(), key, req); err != nil {
		log.Printf("[config] update feature flag error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update feature flag")
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "flag updated"})
}

// GET /api/config/flags/:key/check
func (h *ConfigHandler) CheckFlag(c echo.Context) error {
	key := c.Param("key")
	userID, _ := c.Get("user_id").(int)
	userRoleIDs, _ := c.Get("role_ids").([]int)
	enabled := h.configService.IsFlagEnabled(key, userID, userRoleIDs)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"key":     key,
		"enabled": enabled,
	})
}
