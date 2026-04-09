package services

import (
	"math"
	"testing"

	"wlpr-portal/internal/models"
)

// validatePathCompletion mirrors the completion logic in GetEnrollmentDetail:
//
//	isComplete := requiredComplete >= path.RequiredCount && electiveComplete >= path.ElectiveMin
func validatePathCompletion(requiredCount, electiveMin, requiredDone, electiveDone int) bool {
	return requiredDone >= requiredCount && electiveDone >= electiveMin
}

func TestPathCompletion_AllRequiredAndElectives(t *testing.T) {
	if !validatePathCompletion(3, 2, 3, 2) {
		t.Error("expected complete when all 3/3 required and 2/2 elective are done")
	}
}

func TestPathCompletion_RequiredNotMet(t *testing.T) {
	if validatePathCompletion(3, 2, 2, 2) {
		t.Error("expected incomplete when only 2/3 required are done")
	}
}

func TestPathCompletion_ElectivesNotMet(t *testing.T) {
	if validatePathCompletion(3, 2, 3, 1) {
		t.Error("expected incomplete when only 1/2 electives are done")
	}
}

func TestPathCompletion_BothNotMet(t *testing.T) {
	if validatePathCompletion(3, 2, 1, 0) {
		t.Error("expected incomplete when only 1/3 required and 0/2 elective are done")
	}
}

func TestPathCompletion_ZeroRequirements(t *testing.T) {
	if !validatePathCompletion(0, 0, 0, 0) {
		t.Error("expected complete when requirements are 0 required and 0 elective")
	}
}

func TestPathCompletion_ExcessElectives(t *testing.T) {
	if !validatePathCompletion(3, 2, 3, 5) {
		t.Error("expected complete when 3/3 required and 5/2 electives (over-completion)")
	}
}

func TestPathCompletion_ExactMatch(t *testing.T) {
	if !validatePathCompletion(5, 3, 5, 3) {
		t.Error("expected complete when exactly meeting minimums: 5/5 required, 3/3 elective")
	}
}

// ---- Recommendation Diversity Tests ----

// intPtr is a test helper to create *int values.
func intPtr(v int) *int { return &v }

// TestDiversityControl_EnforcesMaxCategoryPct verifies that no single category
// exceeds the configured max percentage (40%) of the recommendation set.
func TestDiversityControl_EnforcesMaxCategoryPct(t *testing.T) {
	w := &RecommendationWorker{maxCatPct: 0.40}

	cat1 := intPtr(1)
	cat2 := intPtr(2)
	cat3 := intPtr(3)

	// Create 10 recs: 7 from cat1, 2 from cat2, 1 from cat3
	// At 40% cap on 10 items, max per category = ceil(10 * 0.4) = 4
	recs := []models.Recommendation{
		{UserID: 1, ResourceID: 1, Score: 100, CategoryID: cat1},
		{UserID: 1, ResourceID: 2, Score: 95, CategoryID: cat1},
		{UserID: 1, ResourceID: 3, Score: 90, CategoryID: cat1},
		{UserID: 1, ResourceID: 4, Score: 85, CategoryID: cat1},
		{UserID: 1, ResourceID: 5, Score: 80, CategoryID: cat1},
		{UserID: 1, ResourceID: 6, Score: 75, CategoryID: cat1},
		{UserID: 1, ResourceID: 7, Score: 70, CategoryID: cat1},
		{UserID: 1, ResourceID: 8, Score: 60, CategoryID: cat2},
		{UserID: 1, ResourceID: 9, Score: 50, CategoryID: cat2},
		{UserID: 1, ResourceID: 10, Score: 40, CategoryID: cat3},
	}

	result := w.applyDiversityControl(recs)

	// Count per category in the result
	catCounts := make(map[int]int)
	for _, rec := range result {
		catKey := 0
		if rec.CategoryID != nil {
			catKey = *rec.CategoryID
		}
		catCounts[catKey]++
	}

	maxAllowed := int(math.Ceil(float64(len(recs)) * w.maxCatPct))
	for catID, count := range catCounts {
		if count > maxAllowed {
			t.Errorf("category %d has %d recs, exceeds max allowed %d (40%% of %d)",
				catID, count, maxAllowed, len(recs))
		}
	}

	// Specifically: cat1 should have at most 4 items
	if catCounts[1] > 4 {
		t.Errorf("category 1 has %d items, expected <= 4", catCounts[1])
	}
}

// TestDiversityControl_DeferredItemsAlsoChecked verifies that deferred items
// do not bypass the category cap during the refill phase.
func TestDiversityControl_DeferredItemsAlsoChecked(t *testing.T) {
	w := &RecommendationWorker{maxCatPct: 0.40}

	cat1 := intPtr(1)

	// Create 5 recs: all from cat1
	// At 40% cap on 5 items, max per category = ceil(5 * 0.4) = 2
	recs := []models.Recommendation{
		{UserID: 1, ResourceID: 1, Score: 100, CategoryID: cat1},
		{UserID: 1, ResourceID: 2, Score: 90, CategoryID: cat1},
		{UserID: 1, ResourceID: 3, Score: 80, CategoryID: cat1},
		{UserID: 1, ResourceID: 4, Score: 70, CategoryID: cat1},
		{UserID: 1, ResourceID: 5, Score: 60, CategoryID: cat1},
	}

	result := w.applyDiversityControl(recs)

	catCount := 0
	for _, rec := range result {
		if rec.CategoryID != nil && *rec.CategoryID == 1 {
			catCount++
		}
	}

	maxAllowed := int(math.Ceil(float64(len(recs)) * w.maxCatPct))
	if catCount > maxAllowed {
		t.Errorf("DIVERSITY VIOLATION: category 1 has %d recs (max allowed %d). "+
			"Deferred items bypassed the category cap.", catCount, maxAllowed)
	}
}

// TestDiversityControl_EmptyInput handles edge case of empty slice.
func TestDiversityControl_EmptyInput(t *testing.T) {
	w := &RecommendationWorker{maxCatPct: 0.40}
	result := w.applyDiversityControl(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d items", len(result))
	}
}

// TestDiversityControl_SingleCategory verifies behavior when all items share one category.
func TestDiversityControl_SingleCategory(t *testing.T) {
	w := &RecommendationWorker{maxCatPct: 0.40}
	cat1 := intPtr(1)

	recs := make([]models.Recommendation, 20)
	for i := range recs {
		recs[i] = models.Recommendation{
			UserID:     1,
			ResourceID: i + 1,
			Score:      float64(100 - i),
			CategoryID: cat1,
		}
	}

	result := w.applyDiversityControl(recs)

	maxAllowed := int(math.Ceil(float64(len(recs)) * w.maxCatPct)) // ceil(20 * 0.4) = 8
	catCount := 0
	for _, rec := range result {
		if rec.CategoryID != nil && *rec.CategoryID == 1 {
			catCount++
		}
	}

	if catCount > maxAllowed {
		t.Errorf("single-category set: got %d items, max allowed %d", catCount, maxAllowed)
	}

	// With only 1 category, result size should equal maxAllowed since no other categories to fill
	if len(result) > maxAllowed {
		t.Errorf("result size %d exceeds max allowed %d for single category", len(result), maxAllowed)
	}
}
