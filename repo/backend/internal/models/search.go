package models

import "time"

type Resource struct {
	ID              int        `json:"id"`
	Title           string     `json:"title"`
	Slug            string     `json:"slug"`
	Description     *string    `json:"description,omitempty"`
	ContentBody     *string    `json:"content_body,omitempty"`
	ResourceType    string     `json:"resource_type"`
	Status          string     `json:"status"`
	CategoryID      *int       `json:"category_id,omitempty"`
	CategoryName    string     `json:"category_name,omitempty"`
	AuthorID        *int       `json:"author_id,omitempty"`
	DurationMins    *int       `json:"duration_mins,omitempty"`
	Difficulty      *string    `json:"difficulty,omitempty"`
	ThumbnailURL    *string    `json:"thumbnail_url,omitempty"`
	ExternalURL     *string    `json:"external_url,omitempty"`
	ViewCount       int        `json:"view_count"`
	CompletionCount int        `json:"completion_count"`
	PopularityScore float64    `json:"popularity_score"`
	PinyinTitle     *string    `json:"pinyin_title,omitempty"`
	ContentHash     *string    `json:"content_hash,omitempty"`
	Tags            []TagBrief `json:"tags,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	// Search-specific fields
	SearchRank      float64 `json:"search_rank,omitempty"`
	TrigramSimilarity float64 `json:"trigram_similarity,omitempty"`
}

type TagBrief struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type SearchRequest struct {
	Query      string   `json:"query" query:"q"`
	Categories []int    `json:"categories" query:"categories"`
	Tags       []int    `json:"tags" query:"tags"`
	DateFrom   string   `json:"date_from" query:"date_from"`
	DateTo     string   `json:"date_to" query:"date_to"`
	Difficulty string   `json:"difficulty" query:"difficulty"`
	Type       string   `json:"type" query:"type"`
	SortBy     string   `json:"sort_by" query:"sort_by"`     // relevance, popularity, recent
	Page       int      `json:"page" query:"page"`
	PageSize   int      `json:"page_size" query:"page_size"`
}

type SearchResponse struct {
	Results    []Resource `json:"results"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	Query      string     `json:"query"`
	Synonyms   []string   `json:"synonyms_applied,omitempty"`
}

type ArchivePage struct {
	Month         string `json:"month"`
	CategoryID    *int   `json:"category_id,omitempty"`
	CategoryName  string `json:"category_name,omitempty"`
	ResourceCount int    `json:"resource_count"`
}
