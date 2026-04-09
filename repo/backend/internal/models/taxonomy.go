package models

import "time"

type TaxonomyTag struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	TagType     string     `json:"tag_type"`
	ParentID    *int       `json:"parent_id,omitempty"`
	CanonicalID *int       `json:"canonical_id,omitempty"`
	Pinyin      *string    `json:"pinyin,omitempty"`
	Description *string    `json:"description,omitempty"`
	IsCanonical bool       `json:"is_canonical"`
	Level       int        `json:"level"`
	SortOrder   int        `json:"sort_order"`
	UsageCount  int        `json:"usage_count"`
	Status      string     `json:"status"`
	Children    []TaxonomyTag `json:"children,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TaxonomySynonym struct {
	ID             int       `json:"id"`
	Term           string    `json:"term"`
	CanonicalTagID int       `json:"canonical_tag_id"`
	CanonicalName  string    `json:"canonical_name,omitempty"`
	Status         string    `json:"status"`
	CreatedBy      *int      `json:"created_by,omitempty"`
	ReviewedBy     *int      `json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateTagRequest struct {
	Name        string  `json:"name" validate:"required"`
	TagType     string  `json:"tag_type" validate:"required"`
	ParentID    *int    `json:"parent_id,omitempty"`
	Pinyin      *string `json:"pinyin,omitempty"`
	Description *string `json:"description,omitempty"`
}

type CreateSynonymRequest struct {
	Term           string `json:"term" validate:"required"`
	CanonicalTagID int    `json:"canonical_tag_id" validate:"required"`
}

type TaxonomyReviewItem struct {
	ID            int        `json:"id"`
	EntityType    string     `json:"entity_type"`
	EntityID      int        `json:"entity_id"`
	Action        *string    `json:"action,omitempty"`
	Reason        *string    `json:"reason,omitempty"`
	SubmittedBy   *int       `json:"submitted_by,omitempty"`
	ReviewedBy    *int       `json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
	DecisionNotes *string    `json:"decision_notes,omitempty"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ReviewActionRequest struct {
	ReviewItemID  int    `json:"review_item_id" validate:"required"`
	Action        string `json:"action" validate:"required"` // approve or reject
	DecisionNotes string `json:"decision_notes,omitempty"`
}
