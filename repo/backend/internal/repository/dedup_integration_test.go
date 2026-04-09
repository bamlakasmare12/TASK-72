// Package repository contains integration tests for the content_hash deduplication pipeline.
// Requires a running PostgreSQL instance with the schema from init.sql applied.
//
// Run with:
//
//	TEST_DATABASE_URL="postgres://wlpr:wlpr_secret@localhost:5432/wlpr_portal?sslmode=disable" \
//	  go test -v -tags=integration ./internal/repository/ -run Integration
//
//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"testing"

	"wlpr-portal/internal/models"
)

// TestIntegration_ContentHashTrigger_AutoGeneratesHash verifies that the PostgreSQL trigger
// compute_content_hash() automatically populates content_hash when a resource is inserted.
func TestIntegration_ContentHashTrigger_AutoGeneratesHash(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	slug := fmt.Sprintf("hash-trigger-test-%d", os.Getpid())

	var resourceID int
	var contentHash *string
	err := db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, content_body, resource_type, status, published_at)
		VALUES (
			'Docker Container Security Best Practices',
			$1,
			'Comprehensive guide to securing Docker containers in production.',
			'This guide covers image scanning, least privilege, network policies.',
			'article',
			'published',
			NOW()
		)
		RETURNING id, content_hash
	`, slug).Scan(&resourceID, &contentHash)
	if err != nil {
		t.Fatalf("failed to insert test resource: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", resourceID)

	if contentHash == nil || *contentHash == "" {
		t.Fatal("content_hash should be auto-computed by the trigger, but it is empty")
	}

	// content_hash should be a 64-character hex string (SHA-256)
	if len(*contentHash) != 64 {
		t.Errorf("content_hash should be 64 hex chars (SHA-256), got length %d: %s", len(*contentHash), *contentHash)
	}

	t.Logf("Auto-generated content_hash: %s", *contentHash)
}

// TestIntegration_ContentHashTrigger_DuplicateContentSameHash verifies that two resources
// with identical title+description+content_body produce the same content_hash.
func TestIntegration_ContentHashTrigger_DuplicateContentSameHash(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	title := "Identical Content Resource"
	description := "Same description text."
	contentBody := "Same body content."

	slug1 := fmt.Sprintf("hash-dup-1-%d", os.Getpid())
	slug2 := fmt.Sprintf("hash-dup-2-%d", os.Getpid())

	var id1, id2 int
	var hash1, hash2 string

	err := db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, content_body, resource_type, status, published_at)
		VALUES ($1, $2, $3, $4, 'article', 'published', NOW())
		RETURNING id, content_hash
	`, title, slug1, description, contentBody).Scan(&id1, &hash1)
	if err != nil {
		t.Fatalf("insert resource 1: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", id1)

	err = db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, content_body, resource_type, status, published_at)
		VALUES ($1, $2, $3, $4, 'article', 'published', NOW())
		RETURNING id, content_hash
	`, title, slug2, description, contentBody).Scan(&id2, &hash2)
	if err != nil {
		t.Fatalf("insert resource 2: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", id2)

	if hash1 != hash2 {
		t.Fatalf("identical content should produce identical hashes, got %s vs %s", hash1, hash2)
	}

	t.Logf("Both resources have matching content_hash: %s", hash1)
}

// TestIntegration_SearchDedup_CollapsesDuplicateResults verifies that the search query
// deduplicates results by content_hash, keeping only the highest-popularity resource per group.
func TestIntegration_SearchDedup_CollapsesDuplicateResults(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	title := "Dedup Test Resource Content"
	description := "Testing near-duplicate deduplication in search."
	contentBody := "Full text body for dedup testing with unique content."

	slug1 := fmt.Sprintf("dedup-search-1-%d", os.Getpid())
	slug2 := fmt.Sprintf("dedup-search-2-%d", os.Getpid())

	// Insert two resources with identical content but different popularity scores
	var id1, id2 int
	err := db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, content_body, resource_type, status, popularity_score, published_at)
		VALUES ($1, $2, $3, $4, 'article', 'published', 100.0, NOW())
		RETURNING id
	`, title, slug1, description, contentBody).Scan(&id1)
	if err != nil {
		t.Fatalf("insert resource 1: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", id1)

	err = db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, content_body, resource_type, status, popularity_score, published_at)
		VALUES ($1, $2, $3, $4, 'article', 'published', 50.0, NOW())
		RETURNING id
	`, title, slug2, description, contentBody).Scan(&id2)
	if err != nil {
		t.Fatalf("insert resource 2: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", id2)

	// Verify both have the same content_hash
	var hash1, hash2 string
	db.QueryRow(ctx, "SELECT content_hash FROM resources WHERE id = $1", id1).Scan(&hash1)
	db.QueryRow(ctx, "SELECT content_hash FROM resources WHERE id = $1", id2).Scan(&hash2)
	if hash1 != hash2 {
		t.Fatalf("test setup error: resources should have same hash, got %s vs %s", hash1, hash2)
	}

	// Run the deduped search query
	searchRepo := NewSearchRepository(db)
	req := models.SearchRequest{
		Query:    "Dedup Test Resource",
		Page:     1,
		PageSize: 20,
		SortBy:   "relevance",
	}

	resp, err := searchRepo.SearchResources(ctx, req, nil, "")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// Count how many of our test resources appear
	matchCount := 0
	for _, r := range resp.Results {
		if r.ID == id1 || r.ID == id2 {
			matchCount++
		}
	}

	if matchCount > 1 {
		t.Errorf("DEDUP FAILURE: both duplicate resources appeared in search results (expected only 1, got %d)", matchCount)
	}
	if matchCount == 1 {
		// Verify the higher-popularity one was kept
		for _, r := range resp.Results {
			if r.ID == id2 {
				t.Error("DEDUP: lower-popularity duplicate (id2) was kept instead of higher-popularity (id1)")
			}
		}
	}

	t.Log("Search dedup correctly collapsed duplicate content_hash resources")
}
