package services

import (
	"testing"
)

// The synonym conflict detection is enforced by a PostgreSQL trigger (check_synonym_conflict),
// so the definitive test is an integration test against a real DB. However, we can validate
// the backend's conflict detection logic rules here as unit tests.

// synonymConflictExists checks whether adding a new active synonym with the given term
// and canonical_tag_id would conflict with existing active synonyms.
// This mirrors the logic in the PostgreSQL trigger check_synonym_conflict().
func synonymConflictExists(existingSynonyms []testSynonym, newTerm string, newCanonicalID int) bool {
	for _, existing := range existingSynonyms {
		if existing.Status != "active" {
			continue
		}
		if existing.Term == newTerm && existing.CanonicalTagID != newCanonicalID {
			return true
		}
	}
	return false
}

type testSynonym struct {
	ID             int
	Term           string
	CanonicalTagID int
	Status         string
}

func TestSynonymConflict_DuplicateActiveBlocksDifferentCanonical(t *testing.T) {
	// Pre-existing: "ML" -> canonical tag 3 (Data Science), active
	existing := []testSynonym{
		{ID: 1, Term: "ML", CanonicalTagID: 3, Status: "active"},
	}

	// Attempt: "ML" -> canonical tag 7 (Go) — MUST conflict
	if !synonymConflictExists(existing, "ML", 7) {
		t.Error("expected conflict: term 'ML' already maps to canonical 3, cannot also map to 7")
	}
}

func TestSynonymConflict_SameCanonicalAllowed(t *testing.T) {
	// Pre-existing: "ML" -> canonical tag 3, active
	existing := []testSynonym{
		{ID: 1, Term: "ML", CanonicalTagID: 3, Status: "active"},
	}

	// Attempt: "ML" -> canonical tag 3 (same tag) — must NOT conflict
	// (this is an upsert/duplicate, not a conflict)
	if synonymConflictExists(existing, "ML", 3) {
		t.Error("should NOT conflict: same term pointing to the same canonical tag is allowed")
	}
}

func TestSynonymConflict_InactiveSynonymDoesNotBlock(t *testing.T) {
	// Pre-existing: "ML" -> canonical tag 3, but INACTIVE (pending_review)
	existing := []testSynonym{
		{ID: 1, Term: "ML", CanonicalTagID: 3, Status: "pending_review"},
	}

	// Attempt: "ML" -> canonical tag 7 — must NOT conflict (existing is inactive)
	if synonymConflictExists(existing, "ML", 7) {
		t.Error("should NOT conflict: existing synonym is not active")
	}
}

func TestSynonymConflict_DifferentTermNoConflict(t *testing.T) {
	// Pre-existing: "ML" -> canonical tag 3, active
	existing := []testSynonym{
		{ID: 1, Term: "ML", CanonicalTagID: 3, Status: "active"},
	}

	// Attempt: "machine learning" -> canonical tag 7 — NOT a conflict (different term)
	if synonymConflictExists(existing, "machine learning", 7) {
		t.Error("should NOT conflict: different term can map to a different canonical tag")
	}
}

func TestSynonymConflict_MultipleExistingSynonyms(t *testing.T) {
	existing := []testSynonym{
		{ID: 1, Term: "ML", CanonicalTagID: 3, Status: "active"},
		{ID: 2, Term: "machine learning", CanonicalTagID: 3, Status: "active"},
		{ID: 3, Term: "AWS", CanonicalTagID: 5, Status: "active"},
		{ID: 4, Term: "py", CanonicalTagID: 6, Status: "active"},
	}

	// "ML" -> 5 conflicts (ML already active for 3)
	if !synonymConflictExists(existing, "ML", 5) {
		t.Error("expected conflict: 'ML' active for canonical 3")
	}

	// "AWS" -> 5 does NOT conflict (same canonical)
	if synonymConflictExists(existing, "AWS", 5) {
		t.Error("should NOT conflict: 'AWS' already mapped to canonical 5")
	}

	// "AWS" -> 7 DOES conflict
	if !synonymConflictExists(existing, "AWS", 7) {
		t.Error("expected conflict: 'AWS' active for canonical 5, cannot map to 7")
	}

	// "terraform" -> 5 does NOT conflict (new term)
	if synonymConflictExists(existing, "terraform", 5) {
		t.Error("should NOT conflict: 'terraform' is a new term")
	}

	// "py" -> 6 does NOT conflict (same canonical)
	if synonymConflictExists(existing, "py", 6) {
		t.Error("should NOT conflict: 'py' already maps to canonical 6")
	}

	// "py" -> 3 DOES conflict
	if !synonymConflictExists(existing, "py", 3) {
		t.Error("expected conflict: 'py' active for canonical 6, cannot map to 3")
	}
}

func TestSynonymConflict_RejectedSynonymDoesNotBlock(t *testing.T) {
	existing := []testSynonym{
		{ID: 1, Term: "ML", CanonicalTagID: 3, Status: "rejected"},
	}

	if synonymConflictExists(existing, "ML", 7) {
		t.Error("should NOT conflict: existing synonym was rejected")
	}
}

func TestSynonymConflict_MixedStatuses(t *testing.T) {
	existing := []testSynonym{
		{ID: 1, Term: "ML", CanonicalTagID: 3, Status: "active"},
		{ID: 2, Term: "ML", CanonicalTagID: 5, Status: "rejected"},     // rejected: should not count
		{ID: 3, Term: "ML", CanonicalTagID: 7, Status: "pending_review"}, // pending: should not count
	}

	// "ML" -> 5 DOES conflict because ID 1 (active, canonical 3) still blocks it
	if !synonymConflictExists(existing, "ML", 5) {
		t.Error("expected conflict: 'ML' has an active mapping to canonical 3")
	}

	// "ML" -> 3 does NOT conflict (same canonical as the active one)
	if synonymConflictExists(existing, "ML", 3) {
		t.Error("should NOT conflict: mapping to same canonical as active synonym")
	}
}

func TestSynonymConflict_EmptyExistingAllowed(t *testing.T) {
	existing := []testSynonym{}

	if synonymConflictExists(existing, "anything", 99) {
		t.Error("should NOT conflict: no existing synonyms")
	}
}

func TestSynonymConflict_CaseSensitivity(t *testing.T) {
	// The PostgreSQL trigger compares term = NEW.term, which is case-sensitive at the DB level.
	// The backend normalizes to lowercase before inserting (in CreateSynonym).
	// This test validates that the conflict check is case-sensitive as written.
	existing := []testSynonym{
		{ID: 1, Term: "ml", CanonicalTagID: 3, Status: "active"},
	}

	// Same term, different case — the Go logic does exact match
	if synonymConflictExists(existing, "ML", 7) {
		t.Error("exact-match logic: 'ML' != 'ml', no conflict at logic level (normalization happens at insert)")
	}

	// Same case — DOES conflict
	if !synonymConflictExists(existing, "ml", 7) {
		t.Error("expected conflict: 'ml' exact match with active synonym for different canonical")
	}
}
