package services_test

import (
	"testing"

	"wlpr-portal/internal/models"
	"wlpr-portal/internal/services"
)

func TestDeduplicateByContentHash_RemovesDuplicates(t *testing.T) {
	w := services.NewRecommendationWorkerWithPct(0.40)

	recs := []models.Recommendation{
		{ResourceID: 1, Score: 100, Reason: "similar_users"},
		{ResourceID: 2, Score: 90, Reason: "similar_users"},
		{ResourceID: 3, Score: 80, Reason: "tag_match"},
	}

	hashMap := map[int]string{
		1: "hash_a",
		2: "hash_a", // duplicate of resource 1
		3: "hash_b",
	}

	result := w.DeduplicateByContentHash(recs, hashMap)

	if len(result) != 2 {
		t.Fatalf("expected 2 results after dedup, got %d", len(result))
	}

	// Resource 1 should be kept (higher score, seen first)
	if result[0].ResourceID != 1 {
		t.Errorf("expected resource 1 first, got %d", result[0].ResourceID)
	}
	// Resource 3 should be kept (different hash)
	if result[1].ResourceID != 3 {
		t.Errorf("expected resource 3 second, got %d", result[1].ResourceID)
	}
}

func TestDeduplicateByContentHash_KeepsUniqueHashes(t *testing.T) {
	w := services.NewRecommendationWorkerWithPct(0.40)

	recs := []models.Recommendation{
		{ResourceID: 1, Score: 100},
		{ResourceID: 2, Score: 90},
		{ResourceID: 3, Score: 80},
	}

	hashMap := map[int]string{
		1: "hash_a",
		2: "hash_b",
		3: "hash_c",
	}

	result := w.DeduplicateByContentHash(recs, hashMap)

	if len(result) != 3 {
		t.Fatalf("all unique hashes should be kept, got %d", len(result))
	}
}

func TestDeduplicateByContentHash_NoHashResources(t *testing.T) {
	w := services.NewRecommendationWorkerWithPct(0.40)

	recs := []models.Recommendation{
		{ResourceID: 1, Score: 100},
		{ResourceID: 2, Score: 90},
	}

	// No hashes in the map: all resources should be kept
	hashMap := map[int]string{}

	result := w.DeduplicateByContentHash(recs, hashMap)

	if len(result) != 2 {
		t.Fatalf("resources without hashes should all be kept, got %d", len(result))
	}
}

func TestDeduplicateByContentHash_EmptyInput(t *testing.T) {
	w := services.NewRecommendationWorkerWithPct(0.40)

	result := w.DeduplicateByContentHash(nil, map[int]string{})

	if result != nil {
		t.Fatalf("nil input should return nil, got %v", result)
	}
}

func TestDeduplicateByContentHash_MixedHashAndNoHash(t *testing.T) {
	w := services.NewRecommendationWorkerWithPct(0.40)

	recs := []models.Recommendation{
		{ResourceID: 1, Score: 100},
		{ResourceID: 2, Score: 90},
		{ResourceID: 3, Score: 80},
		{ResourceID: 4, Score: 70},
	}

	hashMap := map[int]string{
		1: "hash_a",
		3: "hash_a", // duplicate of 1
		// 2 and 4 have no hash
	}

	result := w.DeduplicateByContentHash(recs, hashMap)

	if len(result) != 3 {
		t.Fatalf("expected 3 results (1 deduped, 2 without hash kept), got %d", len(result))
	}

	ids := make(map[int]bool)
	for _, r := range result {
		ids[r.ResourceID] = true
	}
	if !ids[1] {
		t.Error("resource 1 should be kept (first with hash_a)")
	}
	if ids[3] {
		t.Error("resource 3 should be deduped (duplicate hash_a)")
	}
	if !ids[2] || !ids[4] {
		t.Error("resources without hash should be kept")
	}
}

func TestDeduplicateByContentHash_EmptyHashString(t *testing.T) {
	w := services.NewRecommendationWorkerWithPct(0.40)

	recs := []models.Recommendation{
		{ResourceID: 1, Score: 100},
		{ResourceID: 2, Score: 90},
	}

	// Empty string hashes should be treated as "no hash"
	hashMap := map[int]string{
		1: "",
		2: "",
	}

	result := w.DeduplicateByContentHash(recs, hashMap)

	if len(result) != 2 {
		t.Fatalf("empty hash strings should not be treated as duplicates, got %d", len(result))
	}
}

// Diversity control tests are in learning_test.go (TestDiversityControl_*).
