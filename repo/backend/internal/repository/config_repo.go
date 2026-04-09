package repository

import (
	"context"

	"wlpr-portal/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConfigRepository struct {
	db *pgxpool.Pool
}

func NewConfigRepository(db *pgxpool.Pool) *ConfigRepository {
	return &ConfigRepository{db: db}
}

func (r *ConfigRepository) GetAllConfigs(ctx context.Context) ([]models.Config, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, key, value, value_type, COALESCE(module, ''),
			   COALESCE(description, ''), updated_by, created_at, updated_at
		FROM configs ORDER BY module, key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	configs := make([]models.Config, 0)
	for rows.Next() {
		var c models.Config
		if err := rows.Scan(&c.ID, &c.Key, &c.Value, &c.ValueType,
			&c.Module, &c.Description, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (r *ConfigRepository) GetConfigByKey(ctx context.Context, key string) (*models.Config, error) {
	c := &models.Config{}
	err := r.db.QueryRow(ctx, `
		SELECT id, key, value, value_type, COALESCE(module, ''),
			   COALESCE(description, ''), updated_by, created_at, updated_at
		FROM configs WHERE key = $1
	`, key).Scan(&c.ID, &c.Key, &c.Value, &c.ValueType,
		&c.Module, &c.Description, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return c, err
}

func (r *ConfigRepository) UpdateConfig(ctx context.Context, key, value string, updatedBy int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE configs SET value = $2, updated_by = $3, updated_at = NOW()
		WHERE key = $1
	`, key, value, updatedBy)
	return err
}

func (r *ConfigRepository) GetAllFeatureFlags(ctx context.Context) ([]models.FeatureFlag, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, key, COALESCE(description, ''), enabled, rollout_strategy,
			   rollout_percentage, allowed_roles, created_by, created_at, updated_at
		FROM feature_flags ORDER BY key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	flags := make([]models.FeatureFlag, 0)
	for rows.Next() {
		var f models.FeatureFlag
		if err := rows.Scan(&f.ID, &f.Key, &f.Description, &f.Enabled,
			&f.RolloutStrategy, &f.RolloutPercentage, &f.AllowedRoles,
			&f.CreatedBy, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		flags = append(flags, f)
	}
	return flags, rows.Err()
}

func (r *ConfigRepository) GetFeatureFlag(ctx context.Context, key string) (*models.FeatureFlag, error) {
	f := &models.FeatureFlag{}
	err := r.db.QueryRow(ctx, `
		SELECT id, key, COALESCE(description, ''), enabled, rollout_strategy,
			   rollout_percentage, allowed_roles, created_by, created_at, updated_at
		FROM feature_flags WHERE key = $1
	`, key).Scan(&f.ID, &f.Key, &f.Description, &f.Enabled,
		&f.RolloutStrategy, &f.RolloutPercentage, &f.AllowedRoles,
		&f.CreatedBy, &f.CreatedAt, &f.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return f, err
}

func (r *ConfigRepository) UpdateFeatureFlag(ctx context.Context, key string, req models.FeatureFlagUpdateRequest) error {
	if req.Enabled != nil {
		if _, err := r.db.Exec(ctx, `
			UPDATE feature_flags SET enabled = $2, updated_at = NOW() WHERE key = $1
		`, key, *req.Enabled); err != nil {
			return err
		}
	}
	if req.RolloutStrategy != "" {
		if _, err := r.db.Exec(ctx, `
			UPDATE feature_flags SET rollout_strategy = $2, updated_at = NOW() WHERE key = $1
		`, key, req.RolloutStrategy); err != nil {
			return err
		}
	}
	if req.RolloutPercentage != nil {
		if _, err := r.db.Exec(ctx, `
			UPDATE feature_flags SET rollout_percentage = $2, updated_at = NOW() WHERE key = $1
		`, key, *req.RolloutPercentage); err != nil {
			return err
		}
	}
	if req.AllowedRoles != nil {
		if _, err := r.db.Exec(ctx, `
			UPDATE feature_flags SET allowed_roles = $2, updated_at = NOW() WHERE key = $1
		`, key, req.AllowedRoles); err != nil {
			return err
		}
	}
	return nil
}
