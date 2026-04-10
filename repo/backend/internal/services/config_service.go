package services

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"sync"
	"time"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/repository"
)

// ConfigService provides in-memory cached access to configs and feature flags.
type ConfigService struct {
	configRepo *repository.ConfigRepository

	mu      sync.RWMutex
	configs map[string]models.Config
	flags   map[string]models.FeatureFlag
}

func NewConfigService(configRepo *repository.ConfigRepository) *ConfigService {
	return &ConfigService{
		configRepo: configRepo,
		configs:    make(map[string]models.Config),
		flags:      make(map[string]models.FeatureFlag),
	}
}

// NewConfigServiceForTest creates a ConfigService with pre-loaded configs for testing
// without requiring a database connection.
func NewConfigServiceForTest(configs map[string]models.Config) *ConfigService {
	return &ConfigService{
		configs: configs,
		flags:   make(map[string]models.FeatureFlag),
	}
}

// NewConfigServiceWithFlags creates a ConfigService pre-loaded with both configs and
// feature flags. Use this in tests that exercise flag-evaluation logic.
func NewConfigServiceWithFlags(configs map[string]models.Config, flags map[string]models.FeatureFlag) *ConfigService {
	return &ConfigService{
		configs: configs,
		flags:   flags,
	}
}

// LoadAll fetches all configs and flags from DB into the in-memory cache.
func (s *ConfigService) LoadAll(ctx context.Context) error {
	configs, err := s.configRepo.GetAllConfigs(ctx)
	if err != nil {
		return err
	}
	flags, err := s.configRepo.GetAllFeatureFlags(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs = make(map[string]models.Config, len(configs))
	for _, c := range configs {
		s.configs[c.Key] = c
	}

	s.flags = make(map[string]models.FeatureFlag, len(flags))
	for _, f := range flags {
		s.flags[f.Key] = f
	}

	return nil
}

// StartBackgroundSync periodically refreshes the in-memory cache from DB.
func (s *ConfigService) StartBackgroundSync(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.LoadAll(ctx); err != nil {
					log.Printf("[config-sync] failed to reload configs: %v", err)
				}
			}
		}
	}()
}

// GetConfig returns a cached config value. Falls through to DB on cache miss.
func (s *ConfigService) GetConfig(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.configs[key]
	if !ok {
		return "", false
	}
	return c.Value, true
}

// GetConfigUpdatedAt returns the updated_at timestamp for a config key.
// Returns time.Now() if the key is not found (conservative: full grace period).
func (s *ConfigService) GetConfigUpdatedAt(key string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.configs[key]
	if !ok {
		return time.Now()
	}
	return c.UpdatedAt
}

// GetAllConfigs returns all cached configs.
func (s *ConfigService) GetAllConfigs() []models.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.Config, 0, len(s.configs))
	for _, c := range s.configs {
		result = append(result, c)
	}
	return result
}

// GetAllFlags returns all cached feature flags.
func (s *ConfigService) GetAllFlags() []models.FeatureFlag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.FeatureFlag, 0, len(s.flags))
	for _, f := range s.flags {
		result = append(result, f)
	}
	return result
}

// IsFlagEnabled checks if a feature flag is enabled for a given user.
// userID is used for deterministic percentage bucketing.
// userRoleIDs is used for role_based rollout strategy.
func (s *ConfigService) IsFlagEnabled(key string, userID int, userRoleIDs []int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, ok := s.flags[key]
	if !ok || !f.Enabled {
		return false
	}

	switch f.RolloutStrategy {
	case "all":
		return true
	case "disabled":
		return false
	case "role_based":
		if len(userRoleIDs) == 0 {
			return false
		}
		for _, allowed := range f.AllowedRoles {
			for _, userRole := range userRoleIDs {
				if allowed == userRole {
					return true
				}
			}
		}
		return false
	case "percentage":
		if f.RolloutPercentage <= 0 {
			return false
		}
		if f.RolloutPercentage >= 100 {
			return true
		}
		if userID == 0 {
			return false
		}
		// Deterministic bucketing: hash(userID, flagKey) % 100 < rolloutPercentage
		h := fnv.New32a()
		_, _ = h.Write([]byte(fmt.Sprintf("%d:%s", userID, key)))
		bucket := int(h.Sum32() % 100)
		return bucket < f.RolloutPercentage
	default:
		return false
	}
}

// UpdateConfig updates a config in DB and refreshes cache.
func (s *ConfigService) UpdateConfig(ctx context.Context, key, value string, updatedBy int) error {
	if err := s.configRepo.UpdateConfig(ctx, key, value, updatedBy); err != nil {
		return err
	}
	return s.LoadAll(ctx)
}

// UpdateFeatureFlag updates a flag in DB and refreshes cache.
func (s *ConfigService) UpdateFeatureFlag(ctx context.Context, key string, req models.FeatureFlagUpdateRequest) error {
	if err := s.configRepo.UpdateFeatureFlag(ctx, key, req); err != nil {
		return err
	}
	return s.LoadAll(ctx)
}
