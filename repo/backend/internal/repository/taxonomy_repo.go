package repository

import (
	"context"
	"fmt"
	"strings"

	"wlpr-portal/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaxonomyRepository struct {
	db *pgxpool.Pool
}

func NewTaxonomyRepository(db *pgxpool.Pool) *TaxonomyRepository {
	return &TaxonomyRepository{db: db}
}

func (r *TaxonomyRepository) GetAllTags(ctx context.Context, tagType string) ([]models.TaxonomyTag, error) {
	query := `
		SELECT id, name, slug, tag_type, parent_id, canonical_id, pinyin,
			   description, is_canonical, level, sort_order, usage_count, status,
			   created_at, updated_at
		FROM taxonomy_tags
		WHERE status = 'active'
	`
	args := []interface{}{}
	if tagType != "" {
		query += " AND tag_type = $1"
		args = append(args, tagType)
	}
	query += " ORDER BY level, sort_order, name"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTags(rows)
}

func (r *TaxonomyRepository) GetTagByID(ctx context.Context, id int) (*models.TaxonomyTag, error) {
	tag := &models.TaxonomyTag{}
	err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, tag_type, parent_id, canonical_id, pinyin,
			   description, is_canonical, level, sort_order, usage_count, status,
			   created_at, updated_at
		FROM taxonomy_tags WHERE id = $1
	`, id).Scan(
		&tag.ID, &tag.Name, &tag.Slug, &tag.TagType, &tag.ParentID,
		&tag.CanonicalID, &tag.Pinyin, &tag.Description, &tag.IsCanonical,
		&tag.Level, &tag.SortOrder, &tag.UsageCount, &tag.Status,
		&tag.CreatedAt, &tag.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return tag, err
}

func (r *TaxonomyRepository) GetTagChildren(ctx context.Context, parentID int) ([]models.TaxonomyTag, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, slug, tag_type, parent_id, canonical_id, pinyin,
			   description, is_canonical, level, sort_order, usage_count, status,
			   created_at, updated_at
		FROM taxonomy_tags WHERE parent_id = $1 AND status = 'active'
		ORDER BY sort_order, name
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTags(rows)
}

func (r *TaxonomyRepository) GetTagHierarchy(ctx context.Context, tagType string) ([]models.TaxonomyTag, error) {
	// Get all root-level tags (only active)
	query := `
		SELECT id, name, slug, tag_type, parent_id, canonical_id, pinyin,
			   description, is_canonical, level, sort_order, usage_count, status,
			   created_at, updated_at
		FROM taxonomy_tags WHERE parent_id IS NULL AND status = 'active'
	`
	args := []interface{}{}
	if tagType != "" {
		query += " AND tag_type = $1"
		args = append(args, tagType)
	}
	query += " ORDER BY sort_order, name"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roots, err := scanTags(rows)
	if err != nil {
		return nil, err
	}

	// Recursively load children
	for i := range roots {
		if err := r.loadChildren(ctx, &roots[i]); err != nil {
			return nil, err
		}
	}
	return roots, nil
}

func (r *TaxonomyRepository) loadChildren(ctx context.Context, tag *models.TaxonomyTag) error {
	children, err := r.GetTagChildren(ctx, tag.ID)
	if err != nil {
		return err
	}
	tag.Children = children
	for i := range tag.Children {
		if err := r.loadChildren(ctx, &tag.Children[i]); err != nil {
			return err
		}
	}
	return nil
}

// CreateTag inserts a new tag with 'pending' status and creates a review queue entry.
// The tag becomes active only after moderator approval.
func (r *TaxonomyRepository) CreateTag(ctx context.Context, req models.CreateTagRequest, createdBy int) (*models.TaxonomyTag, error) {
	slug := slugify(req.Name)

	var level int
	if req.ParentID != nil {
		var parentLevel int
		err := r.db.QueryRow(ctx, "SELECT level FROM taxonomy_tags WHERE id = $1", *req.ParentID).Scan(&parentLevel)
		if err != nil {
			return nil, fmt.Errorf("parent tag not found")
		}
		level = parentLevel + 1
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag := &models.TaxonomyTag{}
	err = tx.QueryRow(ctx, `
		INSERT INTO taxonomy_tags (name, slug, tag_type, parent_id, pinyin, description, level, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8)
		RETURNING id, name, slug, tag_type, parent_id, canonical_id, pinyin,
				  description, is_canonical, level, sort_order, usage_count, status, created_at, updated_at
	`, req.Name, slug, req.TagType, req.ParentID, req.Pinyin, req.Description, level, createdBy).Scan(
		&tag.ID, &tag.Name, &tag.Slug, &tag.TagType, &tag.ParentID,
		&tag.CanonicalID, &tag.Pinyin, &tag.Description, &tag.IsCanonical,
		&tag.Level, &tag.SortOrder, &tag.UsageCount, &tag.Status, &tag.CreatedAt, &tag.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Create review queue entry
	_, err = tx.Exec(ctx, `
		INSERT INTO taxonomy_review_queue (entity_type, entity_id, action, reason, submitted_by, status)
		VALUES ('tag', $1, 'approve', 'New tag creation', $2, 'pending')
	`, tag.ID, createdBy)
	if err != nil {
		return nil, fmt.Errorf("create review queue entry: %w", err)
	}

	// Audit log
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (user_id, action, module, entity_type, entity_id, new_value)
		VALUES ($1, 'taxonomy_tag_submitted', 'taxonomy', 'tag', $2, $3)
	`, createdBy, tag.ID, fmt.Sprintf(`{"name":"%s","tag_type":"%s","status":"pending"}`, tag.Name, tag.TagType))
	if err != nil {
		return nil, fmt.Errorf("audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return tag, nil
}

// Synonym management

func (r *TaxonomyRepository) GetSynonyms(ctx context.Context, canonicalTagID int) ([]models.TaxonomySynonym, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.term, s.canonical_tag_id, t.name, s.status,
			   s.created_by, s.reviewed_by, s.reviewed_at, s.created_at
		FROM taxonomy_synonyms s
		JOIN taxonomy_tags t ON t.id = s.canonical_tag_id
		WHERE s.canonical_tag_id = $1
		ORDER BY s.term
	`, canonicalTagID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	synonyms := make([]models.TaxonomySynonym, 0)
	for rows.Next() {
		var s models.TaxonomySynonym
		if err := rows.Scan(&s.ID, &s.Term, &s.CanonicalTagID, &s.CanonicalName,
			&s.Status, &s.CreatedBy, &s.ReviewedBy, &s.ReviewedAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		synonyms = append(synonyms, s)
	}
	return synonyms, rows.Err()
}

func (r *TaxonomyRepository) GetAllActiveSynonyms(ctx context.Context) ([]models.TaxonomySynonym, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.term, s.canonical_tag_id, t.name, s.status,
			   s.created_by, s.reviewed_by, s.reviewed_at, s.created_at
		FROM taxonomy_synonyms s
		JOIN taxonomy_tags t ON t.id = s.canonical_tag_id
		WHERE s.status = 'active'
		ORDER BY s.term
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	synonyms := make([]models.TaxonomySynonym, 0)
	for rows.Next() {
		var s models.TaxonomySynonym
		if err := rows.Scan(&s.ID, &s.Term, &s.CanonicalTagID, &s.CanonicalName,
			&s.Status, &s.CreatedBy, &s.ReviewedBy, &s.ReviewedAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		synonyms = append(synonyms, s)
	}
	return synonyms, rows.Err()
}

// CreateSynonym creates a synonym with 'pending_review' status and submits it to the review queue.
// The synonym becomes active only after moderator approval.
// The DB trigger check_synonym_conflict fires on status change to 'active'.
func (r *TaxonomyRepository) CreateSynonym(ctx context.Context, term string, canonicalTagID, createdBy int) (*models.TaxonomySynonym, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	s := &models.TaxonomySynonym{}
	err = tx.QueryRow(ctx, `
		INSERT INTO taxonomy_synonyms (term, canonical_tag_id, status, created_by)
		VALUES ($1, $2, 'pending_review', $3)
		RETURNING id, term, canonical_tag_id, status, created_by, created_at
	`, strings.ToLower(strings.TrimSpace(term)), canonicalTagID, createdBy).Scan(
		&s.ID, &s.Term, &s.CanonicalTagID, &s.Status, &s.CreatedBy, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Create review queue entry
	_, err = tx.Exec(ctx, `
		INSERT INTO taxonomy_review_queue (entity_type, entity_id, action, reason, submitted_by, status)
		VALUES ('synonym', $1, 'approve', 'New synonym creation', $2, 'pending')
	`, s.ID, createdBy)
	if err != nil {
		return nil, fmt.Errorf("create review queue entry: %w", err)
	}

	// Audit log
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (user_id, action, module, entity_type, entity_id, new_value)
		VALUES ($1, 'taxonomy_synonym_submitted', 'taxonomy', 'synonym', $2, $3)
	`, createdBy, s.ID, fmt.Sprintf(`{"term":"%s","canonical_tag_id":%d,"status":"pending_review"}`, s.Term, s.CanonicalTagID))
	if err != nil {
		return nil, fmt.Errorf("audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s, nil
}

// FindSynonymMappings looks up active synonym mappings for a search term.
// Returns canonical tag IDs that the term maps to.
func (r *TaxonomyRepository) FindSynonymMappings(ctx context.Context, term string) ([]int, error) {
	lowerTerm := strings.ToLower(strings.TrimSpace(term))
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT canonical_tag_id
		FROM taxonomy_synonyms
		WHERE status = 'active' AND lower(term) = $1
	`, lowerTerm)
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

// Review queue

func (r *TaxonomyRepository) GetReviewQueue(ctx context.Context) ([]models.TaxonomyReviewItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, entity_type, entity_id, action, reason, submitted_by,
			   reviewed_by, reviewed_at, decision_notes, status, created_at
		FROM taxonomy_review_queue
		WHERE status = 'pending'
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.TaxonomyReviewItem, 0)
	for rows.Next() {
		var item models.TaxonomyReviewItem
		if err := rows.Scan(&item.ID, &item.EntityType, &item.EntityID,
			&item.Action, &item.Reason, &item.SubmittedBy,
			&item.ReviewedBy, &item.ReviewedAt, &item.DecisionNotes,
			&item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetReviewQueueAll returns all review queue items (including approved/rejected) for audit purposes.
func (r *TaxonomyRepository) GetReviewQueueAll(ctx context.Context) ([]models.TaxonomyReviewItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, entity_type, entity_id, action, reason, submitted_by,
			   reviewed_by, reviewed_at, decision_notes, status, created_at
		FROM taxonomy_review_queue
		ORDER BY created_at DESC
		LIMIT 200
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.TaxonomyReviewItem, 0)
	for rows.Next() {
		var item models.TaxonomyReviewItem
		if err := rows.Scan(&item.ID, &item.EntityType, &item.EntityID,
			&item.Action, &item.Reason, &item.SubmittedBy,
			&item.ReviewedBy, &item.ReviewedAt, &item.DecisionNotes,
			&item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ApproveReviewItem approves a pending review queue item:
// - Sets the associated tag/synonym status to 'active'
// - Updates the review queue entry with reviewer info
// - Writes audit log entries
func (r *TaxonomyRepository) ApproveReviewItem(ctx context.Context, reviewItemID, reviewerID int, notes string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Fetch the review item
	var entityType string
	var entityID int
	var status string
	err = tx.QueryRow(ctx, `
		SELECT entity_type, entity_id, status
		FROM taxonomy_review_queue WHERE id = $1
	`, reviewItemID).Scan(&entityType, &entityID, &status)
	if err != nil {
		return fmt.Errorf("review item not found: %w", err)
	}
	if status != "pending" {
		return fmt.Errorf("review item is not pending (current status: %s)", status)
	}

	// Activate the entity
	switch entityType {
	case "tag":
		_, err = tx.Exec(ctx, `
			UPDATE taxonomy_tags SET status = 'active', updated_at = NOW() WHERE id = $1
		`, entityID)
	case "synonym":
		_, err = tx.Exec(ctx, `
			UPDATE taxonomy_synonyms SET status = 'active', reviewed_by = $2, reviewed_at = NOW() WHERE id = $1
		`, entityID, reviewerID)
	default:
		return fmt.Errorf("unknown entity type: %s", entityType)
	}
	if err != nil {
		return fmt.Errorf("activate entity: %w", err)
	}

	// Update review queue entry
	_, err = tx.Exec(ctx, `
		UPDATE taxonomy_review_queue
		SET status = 'approved', reviewed_by = $2, reviewed_at = NOW(), decision_notes = $3
		WHERE id = $1
	`, reviewItemID, reviewerID, notes)
	if err != nil {
		return fmt.Errorf("update review queue: %w", err)
	}

	// Audit log
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (user_id, action, module, entity_type, entity_id, new_value)
		VALUES ($1, 'taxonomy_review_approved', 'taxonomy', $2, $3, $4)
	`, reviewerID, entityType, entityID,
		fmt.Sprintf(`{"review_item_id":%d,"decision":"approved","notes":"%s"}`, reviewItemID, notes))
	if err != nil {
		return fmt.Errorf("audit log: %w", err)
	}

	return tx.Commit(ctx)
}

// RejectReviewItem rejects a pending review queue item:
// - Sets the associated tag status to 'rejected' or synonym status to 'rejected'
// - Updates the review queue entry with reviewer info
// - Writes audit log entries
func (r *TaxonomyRepository) RejectReviewItem(ctx context.Context, reviewItemID, reviewerID int, notes string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Fetch the review item
	var entityType string
	var entityID int
	var status string
	err = tx.QueryRow(ctx, `
		SELECT entity_type, entity_id, status
		FROM taxonomy_review_queue WHERE id = $1
	`, reviewItemID).Scan(&entityType, &entityID, &status)
	if err != nil {
		return fmt.Errorf("review item not found: %w", err)
	}
	if status != "pending" {
		return fmt.Errorf("review item is not pending (current status: %s)", status)
	}

	// Reject the entity
	switch entityType {
	case "tag":
		_, err = tx.Exec(ctx, `
			UPDATE taxonomy_tags SET status = 'rejected', updated_at = NOW() WHERE id = $1
		`, entityID)
	case "synonym":
		_, err = tx.Exec(ctx, `
			UPDATE taxonomy_synonyms SET status = 'rejected', reviewed_by = $2, reviewed_at = NOW() WHERE id = $1
		`, entityID, reviewerID)
	default:
		return fmt.Errorf("unknown entity type: %s", entityType)
	}
	if err != nil {
		return fmt.Errorf("reject entity: %w", err)
	}

	// Update review queue entry
	_, err = tx.Exec(ctx, `
		UPDATE taxonomy_review_queue
		SET status = 'rejected', reviewed_by = $2, reviewed_at = NOW(), decision_notes = $3
		WHERE id = $1
	`, reviewItemID, reviewerID, notes)
	if err != nil {
		return fmt.Errorf("update review queue: %w", err)
	}

	// Audit log
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (user_id, action, module, entity_type, entity_id, new_value)
		VALUES ($1, 'taxonomy_review_rejected', 'taxonomy', $2, $3, $4)
	`, reviewerID, entityType, entityID,
		fmt.Sprintf(`{"review_item_id":%d,"decision":"rejected","notes":"%s"}`, reviewItemID, notes))
	if err != nil {
		return fmt.Errorf("audit log: %w", err)
	}

	return tx.Commit(ctx)
}

// Helpers

func scanTags(rows pgx.Rows) ([]models.TaxonomyTag, error) {
	tags := make([]models.TaxonomyTag, 0)
	for rows.Next() {
		var t models.TaxonomyTag
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Slug, &t.TagType, &t.ParentID,
			&t.CanonicalID, &t.Pinyin, &t.Description, &t.IsCanonical,
			&t.Level, &t.SortOrder, &t.UsageCount, &t.Status,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r == ' ' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	// Remove consecutive dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
