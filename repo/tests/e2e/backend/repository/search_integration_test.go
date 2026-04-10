// Package repository_test contains integration tests for the FTS trigger.
// Requires a running PostgreSQL instance with the schema from init.sql applied.
//
// Run with:
//
//	TEST_DATABASE_URL="postgres://wlpr:wlpr_secret@localhost:5432/wlpr_portal?sslmode=disable" \
//	  go test -v -tags=integration ./tests/repository/ -run Integration
//
//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// TestIntegration_FTSTrigger_PopulatesSearchVector tests that the PostgreSQL trigger
// trg_resources_search_vector automatically populates the search_vector column
// when a new resource is inserted.
func TestIntegration_FTSTrigger_PopulatesSearchVector(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	slug := fmt.Sprintf("fts-trigger-test-%d", os.Getpid())

	// Insert a new resource with specific title, description, and content
	var resourceID int
	err := db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, content_body, resource_type, status, published_at)
		VALUES (
			'Kubernetes Deployment Strategies',
			$1,
			'Learn about blue-green, canary, and rolling deployment patterns.',
			'This comprehensive guide covers Kubernetes deployment strategies including blue-green deployments, canary releases, rolling updates, and A/B testing approaches.',
			'article',
			'published',
			NOW()
		)
		RETURNING id
	`, slug).Scan(&resourceID)
	if err != nil {
		t.Fatalf("failed to insert test resource: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", resourceID)

	// Query the search_vector column directly to verify the trigger populated it
	var searchVector string
	err = db.QueryRow(ctx, `
		SELECT search_vector::text FROM resources WHERE id = $1
	`, resourceID).Scan(&searchVector)
	if err != nil {
		t.Fatalf("failed to query search_vector: %v", err)
	}

	if searchVector == "" {
		t.Fatal("search_vector is empty — the trigger trg_resources_search_vector did not fire")
	}

	t.Logf("search_vector content: %s", searchVector)

	// Verify specific tokens exist in the tsvector
	// The title "Kubernetes Deployment Strategies" should produce tokens
	expectedTokens := []string{"kubernetes", "deploy", "strategi"}
	for _, token := range expectedTokens {
		// Check that the tsvector contains the stemmed form of each expected word
		var found bool
		err = db.QueryRow(ctx, `
			SELECT search_vector @@ to_tsquery('english', $2)
			FROM resources WHERE id = $1
		`, resourceID, token+":*").Scan(&found)
		if err != nil {
			t.Fatalf("query for token %q failed: %v", token, err)
		}
		if !found {
			t.Errorf("expected search_vector to match token %q, but it did not", token)
		}
	}

	// Verify description tokens are also indexed (weight B)
	descriptionTokens := []string{"blue", "green", "canari", "roll"}
	for _, token := range descriptionTokens {
		var found bool
		err = db.QueryRow(ctx, `
			SELECT search_vector @@ to_tsquery('english', $2)
			FROM resources WHERE id = $1
		`, resourceID, token+":*").Scan(&found)
		if err != nil {
			t.Fatalf("query for description token %q failed: %v", token, err)
		}
		if !found {
			t.Errorf("expected search_vector to match description token %q", token)
		}
	}

	// Verify content body tokens are indexed (weight C)
	contentTokens := []string{"comprehens", "guid", "releas"}
	for _, token := range contentTokens {
		var found bool
		err = db.QueryRow(ctx, `
			SELECT search_vector @@ to_tsquery('english', $2)
			FROM resources WHERE id = $1
		`, resourceID, token+":*").Scan(&found)
		if err != nil {
			t.Fatalf("query for content token %q failed: %v", token, err)
		}
		if !found {
			t.Errorf("expected search_vector to match content token %q", token)
		}
	}

	t.Log("All FTS tokens verified successfully in search_vector")
}

// TestIntegration_FTSTrigger_UpdateRefreshesVector tests that updating a resource's
// title causes the trigger to re-compute the search_vector.
func TestIntegration_FTSTrigger_UpdateRefreshesVector(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	slug := fmt.Sprintf("fts-update-test-%d", os.Getpid())

	var resourceID int
	err := db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, resource_type, status, published_at)
		VALUES ('Original Title About Python', $1, 'Python programming basics.', 'article', 'published', NOW())
		RETURNING id
	`, slug).Scan(&resourceID)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", resourceID)

	// Verify "python" is in the initial search_vector
	var matchesPython bool
	err = db.QueryRow(ctx, `
		SELECT search_vector @@ to_tsquery('english', 'python')
		FROM resources WHERE id = $1
	`, resourceID).Scan(&matchesPython)
	if err != nil {
		t.Fatalf("initial query failed: %v", err)
	}
	if !matchesPython {
		t.Fatal("initial search_vector should match 'python'")
	}

	// Update the title to something completely different
	_, err = db.Exec(ctx, `
		UPDATE resources SET title = 'Advanced Rust Programming Guide' WHERE id = $1
	`, resourceID)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Now "rust" should be in the vector and "python" should still be there
	// (because "python" is still in the description)
	var matchesRust bool
	err = db.QueryRow(ctx, `
		SELECT search_vector @@ to_tsquery('english', 'rust')
		FROM resources WHERE id = $1
	`, resourceID).Scan(&matchesRust)
	if err != nil {
		t.Fatalf("post-update query for rust failed: %v", err)
	}
	if !matchesRust {
		t.Fatal("after title update, search_vector should match 'rust'")
	}

	// Verify "python" still matches (from description)
	err = db.QueryRow(ctx, `
		SELECT search_vector @@ to_tsquery('english', 'python')
		FROM resources WHERE id = $1
	`, resourceID).Scan(&matchesPython)
	if err != nil {
		t.Fatalf("post-update query for python failed: %v", err)
	}
	if !matchesPython {
		t.Fatal("'python' should still match from the description field")
	}

	t.Log("FTS trigger correctly updates search_vector on title change")
}

// TestIntegration_FTSTrigger_PinyinIndexed tests that pinyin_title content
// is indexed in the search_vector (weight D).
func TestIntegration_FTSTrigger_PinyinIndexed(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	slug := fmt.Sprintf("fts-pinyin-test-%d", os.Getpid())

	var resourceID int
	err := db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, pinyin_title, resource_type, status, published_at)
		VALUES ('Cloud Computing Guide', $1, 'Cloud basics.', 'yun ji suan zhi nan', 'article', 'published', NOW())
		RETURNING id
	`, slug).Scan(&resourceID)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", resourceID)

	// Verify pinyin tokens are in the search_vector
	// Using 'simple' dictionary matches since pinyin is indexed with setweight(to_tsvector('simple', ...))
	var matchesYun bool
	err = db.QueryRow(ctx, `
		SELECT search_vector @@ to_tsquery('simple', 'yun')
		FROM resources WHERE id = $1
	`, resourceID).Scan(&matchesYun)
	if err != nil {
		t.Fatalf("pinyin query failed: %v", err)
	}
	if !matchesYun {
		t.Fatal("search_vector should contain pinyin token 'yun' from pinyin_title")
	}

	t.Log("Pinyin content correctly indexed in search_vector")
}

// TestIntegration_FTSRanking_TitleWeightedHigher tests that title matches
// rank higher than description-only matches.
func TestIntegration_FTSRanking_TitleWeightedHigher(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Insert two resources: one with "microservices" in title, one only in description
	slug1 := fmt.Sprintf("fts-rank-title-%d", os.Getpid())
	slug2 := fmt.Sprintf("fts-rank-desc-%d", os.Getpid())

	var id1, id2 int
	err := db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, resource_type, status, published_at)
		VALUES ('Microservices Architecture Patterns', $1, 'Design patterns for distributed systems.', 'article', 'published', NOW())
		RETURNING id
	`, slug1).Scan(&id1)
	if err != nil {
		t.Fatalf("insert resource 1: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", id1)

	err = db.QueryRow(ctx, `
		INSERT INTO resources (title, slug, description, resource_type, status, published_at)
		VALUES ('System Design Fundamentals', $1, 'Covers microservices and monolithic patterns.', 'article', 'published', NOW())
		RETURNING id
	`, slug2).Scan(&id2)
	if err != nil {
		t.Fatalf("insert resource 2: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM resources WHERE id = $1", id2)

	// Query both with ts_rank_cd and verify title match ranks higher
	var rank1, rank2 float64
	err = db.QueryRow(ctx, `
		SELECT ts_rank_cd(search_vector, to_tsquery('english', 'microservic:*'))
		FROM resources WHERE id = $1
	`, id1).Scan(&rank1)
	if err != nil {
		t.Fatalf("rank query 1: %v", err)
	}

	err = db.QueryRow(ctx, `
		SELECT ts_rank_cd(search_vector, to_tsquery('english', 'microservic:*'))
		FROM resources WHERE id = $1
	`, id2).Scan(&rank2)
	if err != nil {
		t.Fatalf("rank query 2: %v", err)
	}

	t.Logf("Title match rank: %f, Description match rank: %f", rank1, rank2)

	if rank1 <= rank2 {
		t.Errorf("title match (rank=%f) should rank higher than description match (rank=%f)", rank1, rank2)
	}
}
