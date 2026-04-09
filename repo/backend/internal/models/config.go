package models

import "time"

type Config struct {
	ID          int       `json:"id"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	ValueType   string    `json:"value_type"`
	Module      string    `json:"module,omitempty"`
	Description string    `json:"description,omitempty"`
	UpdatedBy   *int      `json:"updated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FeatureFlag struct {
	ID                int       `json:"id"`
	Key               string    `json:"key"`
	Description       string    `json:"description,omitempty"`
	Enabled           bool      `json:"enabled"`
	RolloutStrategy   string    `json:"rollout_strategy"`
	RolloutPercentage int       `json:"rollout_percentage"`
	AllowedRoles      []int     `json:"allowed_roles,omitempty"`
	CreatedBy         *int      `json:"created_by,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AppVersion struct {
	ID               int       `json:"id"`
	Version          string    `json:"version"`
	MinSupported     string    `json:"min_supported"`
	ForceUpdate      bool      `json:"force_update"`
	ReadOnlyGraceDays int      `json:"read_only_grace_days"`
	ReleasedAt       time.Time `json:"released_at"`
}

type ConfigUpdateRequest struct {
	Value string `json:"value" validate:"required"`
}

type FeatureFlagUpdateRequest struct {
	Enabled           *bool  `json:"enabled,omitempty"`
	RolloutStrategy   string `json:"rollout_strategy,omitempty"`
	RolloutPercentage *int   `json:"rollout_percentage,omitempty"`
	AllowedRoles      []int  `json:"allowed_roles,omitempty"`
}
