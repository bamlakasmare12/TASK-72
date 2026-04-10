package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wlpr-portal/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SearchRepository struct {
	db *pgxpool.Pool
}

func NewSearchRepository(db *pgxpool.Pool) *SearchRepository {
	return &SearchRepository{db: db}
}

// SearchResources performs full-text search with tsvector ranking, pg_trgm fuzzy matching,
// optional synonym expansion, pinyin matching, multi-dimensional filtering, and sorting.
func (r *SearchRepository) SearchResources(ctx context.Context, req models.SearchRequest, synonymTagIDs []int, pinyinQuery string) (*models.SearchResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	// Base: only published resources
	conditions = append(conditions, "r.status = 'published'")

	// ---- Full-text & fuzzy search ----
	hasQuery := strings.TrimSpace(req.Query) != ""
	var tsQuery string
	if hasQuery {
		// Build tsquery from user input: split words and OR them with prefix matching
		words := strings.Fields(req.Query)
		tsTerms := make([]string, len(words))
		for i, w := range words {
			tsTerms[i] = w + ":*"
		}
		tsQuery = strings.Join(tsTerms, " | ")

		conditions = append(conditions, fmt.Sprintf(
			"(r.search_vector @@ to_tsquery('english', $%d) OR similarity(r.title, $%d) > 0.15",
			argIdx, argIdx+1,
		))
		args = append(args, tsQuery, req.Query)
		argIdx += 2

		// Pinyin fuzzy matching
		if pinyinQuery != "" {
			conditions[len(conditions)-1] += fmt.Sprintf(
				" OR similarity(COALESCE(r.pinyin_title,''), $%d) > 0.15", argIdx,
			)
			args = append(args, pinyinQuery)
			argIdx++
		}

		// Synonym-expanded tag matching
		if len(synonymTagIDs) > 0 {
			placeholders := make([]string, len(synonymTagIDs))
			for i, id := range synonymTagIDs {
				placeholders[i] = fmt.Sprintf("$%d", argIdx)
				args = append(args, id)
				argIdx++
			}
			conditions[len(conditions)-1] += fmt.Sprintf(
				" OR r.id IN (SELECT resource_id FROM resource_tags WHERE tag_id IN (%s))",
				strings.Join(placeholders, ","),
			)
		}

		conditions[len(conditions)-1] += ")" // close the big OR
	}

	// ---- Multi-dimensional filters ----

	// Category filter
	if len(req.Categories) > 0 {
		placeholders := make([]string, len(req.Categories))
		for i, cat := range req.Categories {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, cat)
			argIdx++
		}
		conditions = append(conditions, fmt.Sprintf(
			"r.category_id IN (%s)", strings.Join(placeholders, ","),
		))
	}

	// Tag filter
	if len(req.Tags) > 0 {
		placeholders := make([]string, len(req.Tags))
		for i, tag := range req.Tags {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, tag)
			argIdx++
		}
		conditions = append(conditions, fmt.Sprintf(
			"r.id IN (SELECT resource_id FROM resource_tags WHERE tag_id IN (%s))",
			strings.Join(placeholders, ","),
		))
	}

	// Date range
	if req.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", req.DateFrom); err == nil {
			conditions = append(conditions, fmt.Sprintf("r.published_at >= $%d", argIdx))
			args = append(args, req.DateFrom)
			argIdx++
		}
	}
	if req.DateTo != "" {
		if _, err := time.Parse("2006-01-02", req.DateTo); err == nil {
			conditions = append(conditions, fmt.Sprintf("r.published_at <= $%d::date + INTERVAL '1 day'", argIdx))
			args = append(args, req.DateTo)
			argIdx++
		}
	}

	// Difficulty filter
	if req.Difficulty != "" {
		conditions = append(conditions, fmt.Sprintf("r.difficulty = $%d", argIdx))
		args = append(args, req.Difficulty)
		argIdx++
	}

	// Resource type filter
	if req.Type != "" {
		conditions = append(conditions, fmt.Sprintf("r.resource_type = $%d", argIdx))
		args = append(args, req.Type)
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	// ---- Sorting ----
	// NOTE: orderClause is used in the outer SELECT from the CTE (alias-free),
	// so column references must NOT use a table prefix.
	var orderClause string
	switch req.SortBy {
	case "popularity":
		orderClause = "popularity_score DESC, view_count DESC"
	case "recent":
		orderClause = "published_at DESC NULLS LAST"
	case "relevance":
		if hasQuery {
			orderClause = "search_rank DESC, trgm_sim DESC, popularity_score DESC"
		} else {
			orderClause = "popularity_score DESC"
		}
	default:
		if hasQuery {
			orderClause = "search_rank DESC, trgm_sim DESC, popularity_score DESC"
		} else {
			orderClause = "published_at DESC NULLS LAST"
		}
	}

	// ---- Build rank columns ----
	var rankColumns string
	if hasQuery {
		rankColumns = fmt.Sprintf(`,
			ts_rank_cd(r.search_vector, to_tsquery('english', $1)) AS search_rank,
			GREATEST(
				similarity(r.title, $2)%s
			) AS trgm_sim`,
			func() string {
				if pinyinQuery != "" {
					return fmt.Sprintf(", similarity(COALESCE(r.pinyin_title,''), $3)")
				}
				return ""
			}(),
		)
	} else {
		rankColumns = ", 0::float AS search_rank, 0::float AS trgm_sim"
	}

	// ---- Count query (with dedup by content_hash) ----
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT r.id,
				ROW_NUMBER() OVER (
					PARTITION BY COALESCE(r.content_hash, r.id::text)
					ORDER BY r.popularity_score DESC, r.id ASC
				) AS dedup_rank
			FROM resources r
			WHERE %s
		) deduped WHERE dedup_rank = 1`, whereClause)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count query failed: %w", err)
	}

	// ---- Main query ----
	offset := (req.Page - 1) * req.PageSize

	// Use a CTE to deduplicate near-duplicate resources by content_hash.
	// Within each content_hash group, keep the resource with the highest popularity_score.
	mainQuery := fmt.Sprintf(`
		WITH ranked AS (
			SELECT r.id, r.title, r.slug, r.description, r.resource_type, r.status,
				   r.category_id, COALESCE(cat.name, '') AS category_name,
				   r.duration_mins, r.difficulty, r.thumbnail_url, r.external_url,
				   r.view_count, r.completion_count, r.popularity_score,
				   r.published_at, r.created_at, r.updated_at,
				   r.content_hash
				   %s,
				   ROW_NUMBER() OVER (
					   PARTITION BY COALESCE(r.content_hash, r.id::text)
					   ORDER BY r.popularity_score DESC, r.id ASC
				   ) AS dedup_rank
			FROM resources r
			LEFT JOIN taxonomy_tags cat ON cat.id = r.category_id
			WHERE %s
		)
		SELECT id, title, slug, description, resource_type, status,
			   category_id, category_name,
			   duration_mins, difficulty, thumbnail_url, external_url,
			   view_count, completion_count, popularity_score,
			   published_at, created_at, updated_at,
			   search_rank, trgm_sim
		FROM ranked
		WHERE dedup_rank = 1
		ORDER BY %s
		LIMIT %d OFFSET %d
	`, rankColumns, whereClause, orderClause, req.PageSize, offset)

	rows, err := r.db.Query(ctx, mainQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	results := make([]models.Resource, 0)
	for rows.Next() {
		var res models.Resource
		var searchRank, trgmSim float64
		if err := rows.Scan(
			&res.ID, &res.Title, &res.Slug, &res.Description,
			&res.ResourceType, &res.Status, &res.CategoryID, &res.CategoryName,
			&res.DurationMins, &res.Difficulty, &res.ThumbnailURL, &res.ExternalURL,
			&res.ViewCount, &res.CompletionCount, &res.PopularityScore,
			&res.PublishedAt, &res.CreatedAt, &res.UpdatedAt,
			&searchRank, &trgmSim,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		res.SearchRank = searchRank
		res.TrigramSimilarity = trgmSim
		results = append(results, res)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load tags for each result
	for i := range results {
		tags, err := r.GetResourceTags(ctx, results[i].ID)
		if err == nil {
			results[i].Tags = tags
		}
	}

	return &models.SearchResponse{
		Results:  results,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Query:    req.Query,
	}, nil
}

func (r *SearchRepository) GetResourceTags(ctx context.Context, resourceID int) ([]models.TagBrief, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.name, t.slug
		FROM taxonomy_tags t
		JOIN resource_tags rt ON rt.tag_id = t.id
		WHERE rt.resource_id = $1
		ORDER BY t.name
	`, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]models.TagBrief, 0)
	for rows.Next() {
		var t models.TagBrief
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *SearchRepository) GetResourceByID(ctx context.Context, id int) (*models.Resource, error) {
	res := &models.Resource{}
	err := r.db.QueryRow(ctx, `
		SELECT r.id, r.title, r.slug, r.description, r.content_body,
			   r.resource_type, r.status, r.category_id,
			   COALESCE(cat.name, '') AS category_name,
			   r.author_id, r.duration_mins, r.difficulty,
			   r.thumbnail_url, r.external_url,
			   r.view_count, r.completion_count, r.popularity_score,
			   r.pinyin_title, r.content_hash,
			   r.published_at, r.created_at, r.updated_at
		FROM resources r
		LEFT JOIN taxonomy_tags cat ON cat.id = r.category_id
		WHERE r.id = $1
	`, id).Scan(
		&res.ID, &res.Title, &res.Slug, &res.Description, &res.ContentBody,
		&res.ResourceType, &res.Status, &res.CategoryID, &res.CategoryName,
		&res.AuthorID, &res.DurationMins, &res.Difficulty,
		&res.ThumbnailURL, &res.ExternalURL,
		&res.ViewCount, &res.CompletionCount, &res.PopularityScore,
		&res.PinyinTitle, &res.ContentHash,
		&res.PublishedAt, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	tags, _ := r.GetResourceTags(ctx, res.ID)
	res.Tags = tags
	return res, nil
}

func (r *SearchRepository) IncrementViewCount(ctx context.Context, resourceID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE resources SET view_count = view_count + 1, updated_at = NOW()
		WHERE id = $1
	`, resourceID)
	return err
}

func (r *SearchRepository) GetArchiveMonthly(ctx context.Context) ([]models.ArchivePage, error) {
	// Refresh materialized view first
	_, _ = r.db.Exec(ctx, "REFRESH MATERIALIZED VIEW resource_archive_monthly")

	rows, err := r.db.Query(ctx, `
		SELECT to_char(a.archive_month, 'YYYY-MM') AS month,
			   a.category_id,
			   COALESCE(t.name, 'Uncategorized') AS category_name,
			   a.resource_count
		FROM resource_archive_monthly a
		LEFT JOIN taxonomy_tags t ON t.id = a.category_id
		ORDER BY a.archive_month DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pages := make([]models.ArchivePage, 0)
	for rows.Next() {
		var p models.ArchivePage
		if err := rows.Scan(&p.Month, &p.CategoryID, &p.CategoryName, &p.ResourceCount); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// RecordEvent inserts a resource interaction event used by the recommendation engine.
func (r *SearchRepository) RecordEvent(ctx context.Context, userID, resourceID int, eventType string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO resource_events (user_id, resource_id, event_type)
		VALUES ($1, $2, $3)
	`, userID, resourceID, eventType)
	return err
}

// RefreshArchiveViews refreshes both monthly and by-tag materialized views.
func (r *SearchRepository) RefreshArchiveViews(ctx context.Context) (int, error) {
	_, err := r.db.Exec(ctx, "REFRESH MATERIALIZED VIEW resource_archive_monthly")
	if err != nil {
		return 0, fmt.Errorf("monthly archive refresh: %w", err)
	}
	_, err = r.db.Exec(ctx, "REFRESH MATERIALIZED VIEW resource_archive_by_tag")
	if err != nil {
		return 0, fmt.Errorf("tag archive refresh: %w", err)
	}
	return 1, nil
}

// CreateResource inserts a new resource and returns its ID.
func (r *SearchRepository) CreateResource(ctx context.Context, res models.Resource, tagIDs []int) (*models.Resource, error) {
	slug := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(res.Title)), " ", "-")
	slug = fmt.Sprintf("%s-%d", slug, time.Now().UnixMilli())

	var id int
	err := r.db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, content_body, resource_type, status,
			category_id, author_id, duration_mins, difficulty, thumbnail_url, external_url,
			pinyin_title, published_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id`,
		res.Title, slug, res.Description, res.ContentBody, res.ResourceType, "published",
		res.CategoryID, res.AuthorID, res.DurationMins, res.Difficulty,
		res.ThumbnailURL, res.ExternalURL, res.PinyinTitle, time.Now(),
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert resource: %w", err)
	}

	for _, tagID := range tagIDs {
		_, _ = r.db.Exec(ctx, `INSERT INTO resource_tags (resource_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, tagID)
	}

	res.ID = id
	res.Slug = slug
	res.Status = "published"
	return &res, nil
}
