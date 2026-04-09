package repository

import (
	"context"
	"fmt"
	"time"

	"wlpr-portal/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LearningRepository struct {
	db *pgxpool.Pool
}

func NewLearningRepository(db *pgxpool.Pool) *LearningRepository {
	return &LearningRepository{db: db}
}

// ---- Learning Paths ----

func (r *LearningRepository) GetAllPaths(ctx context.Context) ([]models.LearningPath, error) {
	rows, err := r.db.Query(ctx, `
		SELECT lp.id, lp.title, lp.slug, lp.description, lp.category_id,
			   COALESCE(t.name, '') AS cat_name,
			   lp.target_job_family, lp.required_count, lp.elective_min,
			   lp.estimated_hours, lp.difficulty, lp.is_active,
			   lp.created_at, lp.updated_at
		FROM learning_paths lp
		LEFT JOIN taxonomy_tags t ON t.id = lp.category_id
		WHERE lp.is_active = TRUE
		ORDER BY lp.title
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPaths(rows)
}

func (r *LearningRepository) GetPathByID(ctx context.Context, id int) (*models.LearningPath, error) {
	path := &models.LearningPath{}
	err := r.db.QueryRow(ctx, `
		SELECT lp.id, lp.title, lp.slug, lp.description, lp.category_id,
			   COALESCE(t.name, '') AS cat_name,
			   lp.target_job_family, lp.required_count, lp.elective_min,
			   lp.estimated_hours, lp.difficulty, lp.is_active,
			   lp.created_at, lp.updated_at
		FROM learning_paths lp
		LEFT JOIN taxonomy_tags t ON t.id = lp.category_id
		WHERE lp.id = $1
	`, id).Scan(
		&path.ID, &path.Title, &path.Slug, &path.Description, &path.CategoryID,
		&path.CategoryName, &path.TargetJobFamily, &path.RequiredCount, &path.ElectiveMin,
		&path.EstimatedHours, &path.Difficulty, &path.IsActive,
		&path.CreatedAt, &path.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	items, err := r.GetPathItems(ctx, id)
	if err != nil {
		return nil, err
	}
	path.Items = items
	return path, nil
}

func (r *LearningRepository) GetPathItems(ctx context.Context, pathID int) ([]models.PathItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT lpi.id, lpi.path_id, lpi.resource_id, lpi.item_type, lpi.sort_order,
			   lpi.created_at,
			   res.id, res.title, res.slug, res.description, res.resource_type,
			   res.duration_mins, res.difficulty, res.thumbnail_url,
			   res.view_count, res.completion_count, res.popularity_score,
			   res.published_at
		FROM learning_path_items lpi
		JOIN resources res ON res.id = lpi.resource_id
		WHERE lpi.path_id = $1
		ORDER BY lpi.sort_order
	`, pathID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.PathItem, 0)
	for rows.Next() {
		var item models.PathItem
		var res models.Resource
		if err := rows.Scan(
			&item.ID, &item.PathID, &item.ResourceID, &item.ItemType, &item.SortOrder,
			&item.CreatedAt,
			&res.ID, &res.Title, &res.Slug, &res.Description, &res.ResourceType,
			&res.DurationMins, &res.Difficulty, &res.ThumbnailURL,
			&res.ViewCount, &res.CompletionCount, &res.PopularityScore,
			&res.PublishedAt,
		); err != nil {
			return nil, err
		}
		item.Resource = &res
		items = append(items, item)
	}
	return items, rows.Err()
}

// ---- Enrollments ----

func (r *LearningRepository) GetUserEnrollments(ctx context.Context, userID int) ([]models.UserEnrollment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ue.id, ue.user_id, ue.path_id, ue.status,
			   ue.enrolled_at, ue.completed_at, ue.last_accessed
		FROM user_enrollments ue
		WHERE ue.user_id = $1
		ORDER BY ue.last_accessed DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	enrollments := make([]models.UserEnrollment, 0)
	for rows.Next() {
		var e models.UserEnrollment
		if err := rows.Scan(&e.ID, &e.UserID, &e.PathID, &e.Status,
			&e.EnrolledAt, &e.CompletedAt, &e.LastAccessed); err != nil {
			return nil, err
		}
		enrollments = append(enrollments, e)
	}
	return enrollments, rows.Err()
}

func (r *LearningRepository) Enroll(ctx context.Context, userID, pathID int) (*models.UserEnrollment, error) {
	e := &models.UserEnrollment{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_enrollments (user_id, path_id, status)
		VALUES ($1, $2, 'active')
		ON CONFLICT (user_id, path_id) DO UPDATE SET
			status = 'active', last_accessed = NOW()
		RETURNING id, user_id, path_id, status, enrolled_at, completed_at, last_accessed
	`, userID, pathID).Scan(
		&e.ID, &e.UserID, &e.PathID, &e.Status,
		&e.EnrolledAt, &e.CompletedAt, &e.LastAccessed,
	)
	return e, err
}

func (r *LearningRepository) DropEnrollment(ctx context.Context, userID, pathID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_enrollments SET status = 'dropped'
		WHERE user_id = $1 AND path_id = $2
	`, userID, pathID)
	return err
}

// ---- Progress ----

func (r *LearningRepository) GetUserProgress(ctx context.Context, userID int, pathID *int) ([]models.UserProgress, error) {
	query := `
		SELECT up.id, up.user_id, up.resource_id, up.path_id, up.status,
			   up.progress_pct, up.time_spent_mins, up.last_position,
			   up.started_at, up.completed_at, up.synced_at,
			   res.id, res.title, res.slug, res.resource_type,
			   res.duration_mins, res.difficulty
		FROM user_progress up
		JOIN resources res ON res.id = up.resource_id
		WHERE up.user_id = $1
	`
	args := []interface{}{userID}
	if pathID != nil {
		query += " AND up.path_id = $2"
		args = append(args, *pathID)
	}
	query += " ORDER BY up.updated_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	progress := make([]models.UserProgress, 0)
	for rows.Next() {
		var p models.UserProgress
		var res models.Resource
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.ResourceID, &p.PathID, &p.Status,
			&p.ProgressPct, &p.TimeSpentMin, &p.LastPosition,
			&p.StartedAt, &p.CompletedAt, &p.SyncedAt,
			&res.ID, &res.Title, &res.Slug, &res.ResourceType,
			&res.DurationMins, &res.Difficulty,
		); err != nil {
			return nil, err
		}
		p.Resource = &res
		progress = append(progress, p)
	}
	return progress, rows.Err()
}

func (r *LearningRepository) UpsertProgress(ctx context.Context, userID int, req models.UpdateProgressRequest) (*models.UserProgress, error) {
	now := time.Now()
	p := &models.UserProgress{}

	var startedAt *time.Time
	if req.Status == "in_progress" || req.Status == "completed" {
		startedAt = &now
	}
	var completedAt *time.Time
	if req.Status == "completed" {
		completedAt = &now
	}

	err := r.db.QueryRow(ctx, `
		INSERT INTO user_progress (user_id, resource_id, path_id, status, progress_pct,
			time_spent_mins, last_position, started_at, completed_at, synced_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		ON CONFLICT (user_id, resource_id, path_id) DO UPDATE SET
			status = EXCLUDED.status,
			progress_pct = EXCLUDED.progress_pct,
			time_spent_mins = user_progress.time_spent_mins + EXCLUDED.time_spent_mins,
			last_position = COALESCE(EXCLUDED.last_position, user_progress.last_position),
			started_at = COALESCE(user_progress.started_at, EXCLUDED.started_at),
			completed_at = COALESCE(EXCLUDED.completed_at, user_progress.completed_at),
			synced_at = NOW(),
			updated_at = NOW()
		RETURNING id, user_id, resource_id, path_id, status, progress_pct,
				  time_spent_mins, last_position, started_at, completed_at, synced_at
	`, userID, req.ResourceID, req.PathID, req.Status, req.ProgressPct,
		req.TimeSpentMin, req.LastPosition, startedAt, completedAt,
	).Scan(
		&p.ID, &p.UserID, &p.ResourceID, &p.PathID, &p.Status,
		&p.ProgressPct, &p.TimeSpentMin, &p.LastPosition,
		&p.StartedAt, &p.CompletedAt, &p.SyncedAt,
	)
	return p, err
}

// GetProgressCountForPath returns completed required/elective counts for a user in a path.
func (r *LearningRepository) GetProgressCountForPath(ctx context.Context, userID, pathID int) (requiredDone, electiveDone int, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN lpi.item_type = 'required' AND up.status = 'completed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN lpi.item_type = 'elective' AND up.status = 'completed' THEN 1 ELSE 0 END), 0)
		FROM learning_path_items lpi
		LEFT JOIN user_progress up ON up.resource_id = lpi.resource_id
			AND up.path_id = lpi.path_id AND up.user_id = $1
		WHERE lpi.path_id = $2
	`, userID, pathID).Scan(&requiredDone, &electiveDone)
	return
}

func (r *LearningRepository) CompleteEnrollment(ctx context.Context, userID, pathID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_enrollments SET status = 'completed', completed_at = NOW()
		WHERE user_id = $1 AND path_id = $2 AND status = 'active'
	`, userID, pathID)
	return err
}

// ---- CSV Export ----

func (r *LearningRepository) GetLearningRecords(ctx context.Context, userID int) ([]models.LearningRecordCSV, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.username, res.title,
			   COALESCE(lp.title, 'Standalone') AS path_title,
			   up.status, up.progress_pct, up.time_spent_mins,
			   COALESCE(to_char(up.started_at, 'YYYY-MM-DD'), ''),
			   COALESCE(to_char(up.completed_at, 'YYYY-MM-DD'), '')
		FROM user_progress up
		JOIN users u ON u.id = up.user_id
		JOIN resources res ON res.id = up.resource_id
		LEFT JOIN learning_paths lp ON lp.id = up.path_id
		WHERE up.user_id = $1
		ORDER BY lp.title NULLS LAST, up.created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]models.LearningRecordCSV, 0)
	for rows.Next() {
		var r models.LearningRecordCSV
		if err := rows.Scan(&r.Username, &r.ResourceTitle, &r.PathTitle,
			&r.Status, &r.ProgressPct, &r.TimeSpentMin,
			&r.StartedAt, &r.CompletedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// ---- Recommendations ----

func (r *LearningRepository) GetRecommendations(ctx context.Context, userID int, limit int) ([]models.Recommendation, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT rec.id, rec.user_id, rec.resource_id, rec.score, rec.reason,
			   rec.category_id, rec.created_at,
			   res.id, res.title, res.slug, res.description, res.resource_type,
			   res.category_id, COALESCE(cat.name, ''),
			   res.duration_mins, res.difficulty, res.thumbnail_url,
			   res.view_count, res.completion_count, res.popularity_score,
			   res.published_at
		FROM recommendations rec
		JOIN resources res ON res.id = rec.resource_id
		LEFT JOIN taxonomy_tags cat ON cat.id = res.category_id
		WHERE rec.user_id = $1 AND res.status = 'published'
		ORDER BY rec.score DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recs := make([]models.Recommendation, 0)
	for rows.Next() {
		var rec models.Recommendation
		var res models.Resource
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.ResourceID, &rec.Score, &rec.Reason,
			&rec.CategoryID, &rec.CreatedAt,
			&res.ID, &res.Title, &res.Slug, &res.Description, &res.ResourceType,
			&res.CategoryID, &res.CategoryName,
			&res.DurationMins, &res.Difficulty, &res.ThumbnailURL,
			&res.ViewCount, &res.CompletionCount, &res.PopularityScore,
			&res.PublishedAt,
		); err != nil {
			return nil, err
		}
		rec.Resource = &res
		recs = append(recs, rec)
	}
	return recs, rows.Err()
}

// UpsertRecommendation inserts or updates a recommendation row.
func (r *LearningRepository) UpsertRecommendation(ctx context.Context, userID, resourceID int, score float64, reason string, categoryID *int, batchID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO recommendations (user_id, resource_id, score, reason, category_id, batch_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, resource_id) DO UPDATE SET
			score = EXCLUDED.score,
			reason = EXCLUDED.reason,
			category_id = EXCLUDED.category_id,
			batch_id = EXCLUDED.batch_id,
			created_at = NOW()
	`, userID, resourceID, score, reason, categoryID, batchID)
	return err
}

// ClearOldRecommendations removes recs from previous batches for a user.
func (r *LearningRepository) ClearOldRecommendations(ctx context.Context, userID int, currentBatchID string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM recommendations
		WHERE user_id = $1 AND (batch_id IS NULL OR batch_id != $2)
	`, userID, currentBatchID)
	return err
}

// GetAllUserIDs returns all active user IDs (for recommendation worker).
func (r *LearningRepository) GetAllUserIDs(ctx context.Context) ([]int, error) {
	rows, err := r.db.Query(ctx, "SELECT id FROM users WHERE is_active = TRUE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetUserEventVector returns a map of resource_id -> interaction_count for a user.
func (r *LearningRepository) GetUserEventVector(ctx context.Context, userID int) (map[int]float64, error) {
	rows, err := r.db.Query(ctx, `
		SELECT resource_id, COUNT(*) AS cnt
		FROM resource_events
		WHERE user_id = $1
		GROUP BY resource_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vec := make(map[int]float64)
	for rows.Next() {
		var rid int
		var cnt float64
		if err := rows.Scan(&rid, &cnt); err != nil {
			return nil, err
		}
		vec[rid] = cnt
	}
	return vec, rows.Err()
}

// GetUserJobFamily returns the job_family for a user.
func (r *LearningRepository) GetUserJobFamily(ctx context.Context, userID int) (string, error) {
	var jf *string
	err := r.db.QueryRow(ctx, "SELECT job_family FROM users WHERE id = $1", userID).Scan(&jf)
	if err != nil || jf == nil {
		return "", err
	}
	return *jf, nil
}

// GetResourcesByJobFamily returns published resources targeting a specific job family.
func (r *LearningRepository) GetResourcesByJobFamily(ctx context.Context, jobFamily string) ([]models.Resource, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT r.id, r.title, r.category_id, r.popularity_score
		FROM resources r
		JOIN resource_tags rt ON rt.resource_id = r.id
		JOIN taxonomy_tags t ON t.id = rt.tag_id
		WHERE r.status = 'published'
		AND (
			t.name ILIKE '%' || $1 || '%'
			OR EXISTS (
				SELECT 1 FROM learning_paths lp
				JOIN learning_path_items lpi ON lpi.path_id = lp.id
				WHERE lpi.resource_id = r.id AND lp.target_job_family = $1
			)
		)
		ORDER BY r.popularity_score DESC
		LIMIT 50
	`, jobFamily)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resources := make([]models.Resource, 0)
	for rows.Next() {
		var r models.Resource
		if err := rows.Scan(&r.ID, &r.Title, &r.CategoryID, &r.PopularityScore); err != nil {
			return nil, err
		}
		resources = append(resources, r)
	}
	return resources, rows.Err()
}

// GetAllPublishedResources returns all published resources with category info and content_hash.
func (r *LearningRepository) GetAllPublishedResources(ctx context.Context) ([]models.Resource, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, title, category_id, popularity_score, content_hash
		FROM resources WHERE status = 'published'
		ORDER BY popularity_score DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resources := make([]models.Resource, 0)
	for rows.Next() {
		var res models.Resource
		if err := rows.Scan(&res.ID, &res.Title, &res.CategoryID, &res.PopularityScore, &res.ContentHash); err != nil {
			return nil, fmt.Errorf("scan resource: %w", err)
		}
		resources = append(resources, res)
	}
	return resources, rows.Err()
}

// Helpers

func scanPaths(rows pgx.Rows) ([]models.LearningPath, error) {
	paths := make([]models.LearningPath, 0)
	for rows.Next() {
		var p models.LearningPath
		if err := rows.Scan(
			&p.ID, &p.Title, &p.Slug, &p.Description, &p.CategoryID,
			&p.CategoryName, &p.TargetJobFamily, &p.RequiredCount, &p.ElectiveMin,
			&p.EstimatedHours, &p.Difficulty, &p.IsActive,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
