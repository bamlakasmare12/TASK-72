// Package repository contains integration tests for the taxonomy review queue workflow.
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

// TestIntegration_TaxonomyReviewQueue_TagWorkflow tests the full tag creation -> review -> approve/reject flow.
func TestIntegration_TaxonomyReviewQueue_TagWorkflow(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	repo := NewTaxonomyRepository(db)

	// Ensure a test user exists (user_id=1 should exist from seed, but just in case)
	// We use createdBy=1 for the submissions and reviewerID=1 for reviews
	createdBy := 1
	reviewerID := 1

	t.Run("CreateTag_StartsPending", func(t *testing.T) {
		req := models.CreateTagRequest{
			Name:    fmt.Sprintf("ReviewTest Tag %d", os.Getpid()),
			TagType: "skill",
		}
		tag, err := repo.CreateTag(ctx, req, createdBy)
		if err != nil {
			t.Fatalf("CreateTag failed: %v", err)
		}
		defer db.Exec(ctx, "DELETE FROM taxonomy_tags WHERE id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM taxonomy_review_queue WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM audit_log WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)

		if tag.Status != "pending" {
			t.Errorf("new tag should have status 'pending', got %q", tag.Status)
		}

		// Verify review queue entry was created
		queue, err := repo.GetReviewQueue(ctx)
		if err != nil {
			t.Fatalf("GetReviewQueue failed: %v", err)
		}
		found := false
		for _, item := range queue {
			if item.EntityType == "tag" && item.EntityID == tag.ID {
				found = true
				if item.Status != "pending" {
					t.Errorf("review queue entry should be 'pending', got %q", item.Status)
				}
			}
		}
		if !found {
			t.Error("review queue entry not found for the new tag")
		}

		// Verify pending tag is NOT returned by GetAllTags (active-only filter)
		allTags, err := repo.GetAllTags(ctx, "skill")
		if err != nil {
			t.Fatalf("GetAllTags failed: %v", err)
		}
		for _, at := range allTags {
			if at.ID == tag.ID {
				t.Error("pending tag should NOT appear in GetAllTags (active-only filter)")
			}
		}

		// Verify audit log entry
		var auditCount int
		db.QueryRow(ctx, `
			SELECT COUNT(*) FROM audit_log
			WHERE entity_type = 'tag' AND entity_id = $1 AND action = 'taxonomy_tag_submitted'
		`, tag.ID).Scan(&auditCount)
		if auditCount == 0 {
			t.Error("audit log entry for tag submission is missing")
		}
	})

	t.Run("ApproveTag_ActivatesTag", func(t *testing.T) {
		req := models.CreateTagRequest{
			Name:    fmt.Sprintf("ApproveTest Tag %d", os.Getpid()),
			TagType: "skill",
		}
		tag, err := repo.CreateTag(ctx, req, createdBy)
		if err != nil {
			t.Fatalf("CreateTag failed: %v", err)
		}
		defer db.Exec(ctx, "DELETE FROM taxonomy_tags WHERE id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM taxonomy_review_queue WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM audit_log WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)

		// Find the review queue item
		queue, _ := repo.GetReviewQueue(ctx)
		var reviewItemID int
		for _, item := range queue {
			if item.EntityType == "tag" && item.EntityID == tag.ID {
				reviewItemID = item.ID
				break
			}
		}
		if reviewItemID == 0 {
			t.Fatal("review queue entry not found")
		}

		// Approve
		err = repo.ApproveReviewItem(ctx, reviewItemID, reviewerID, "Looks good")
		if err != nil {
			t.Fatalf("ApproveReviewItem failed: %v", err)
		}

		// Verify tag is now active
		updatedTag, _ := repo.GetTagByID(ctx, tag.ID)
		if updatedTag.Status != "active" {
			t.Errorf("approved tag should be 'active', got %q", updatedTag.Status)
		}

		// Verify tag now appears in GetAllTags
		allTags, _ := repo.GetAllTags(ctx, "skill")
		found := false
		for _, at := range allTags {
			if at.ID == tag.ID {
				found = true
			}
		}
		if !found {
			t.Error("approved tag should appear in GetAllTags")
		}

		// Verify approval audit log
		var auditCount int
		db.QueryRow(ctx, `
			SELECT COUNT(*) FROM audit_log
			WHERE entity_type = 'tag' AND entity_id = $1 AND action = 'taxonomy_review_approved'
		`, tag.ID).Scan(&auditCount)
		if auditCount == 0 {
			t.Error("audit log entry for tag approval is missing")
		}
	})

	t.Run("RejectTag_RejectsTag", func(t *testing.T) {
		req := models.CreateTagRequest{
			Name:    fmt.Sprintf("RejectTest Tag %d", os.Getpid()),
			TagType: "skill",
		}
		tag, err := repo.CreateTag(ctx, req, createdBy)
		if err != nil {
			t.Fatalf("CreateTag failed: %v", err)
		}
		defer db.Exec(ctx, "DELETE FROM taxonomy_tags WHERE id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM taxonomy_review_queue WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM audit_log WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)

		queue, _ := repo.GetReviewQueue(ctx)
		var reviewItemID int
		for _, item := range queue {
			if item.EntityType == "tag" && item.EntityID == tag.ID {
				reviewItemID = item.ID
				break
			}
		}

		err = repo.RejectReviewItem(ctx, reviewItemID, reviewerID, "Not relevant to our taxonomy")
		if err != nil {
			t.Fatalf("RejectReviewItem failed: %v", err)
		}

		updatedTag, _ := repo.GetTagByID(ctx, tag.ID)
		if updatedTag.Status != "rejected" {
			t.Errorf("rejected tag should be 'rejected', got %q", updatedTag.Status)
		}

		// Rejected tag should NOT appear in GetAllTags
		allTags, _ := repo.GetAllTags(ctx, "skill")
		for _, at := range allTags {
			if at.ID == tag.ID {
				t.Error("rejected tag should NOT appear in GetAllTags")
			}
		}
	})

	t.Run("CannotApproveAlreadyApproved", func(t *testing.T) {
		req := models.CreateTagRequest{
			Name:    fmt.Sprintf("DoubleApprove Tag %d", os.Getpid()),
			TagType: "skill",
		}
		tag, err := repo.CreateTag(ctx, req, createdBy)
		if err != nil {
			t.Fatalf("CreateTag failed: %v", err)
		}
		defer db.Exec(ctx, "DELETE FROM taxonomy_tags WHERE id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM taxonomy_review_queue WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM audit_log WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)

		queue, _ := repo.GetReviewQueue(ctx)
		var reviewItemID int
		for _, item := range queue {
			if item.EntityType == "tag" && item.EntityID == tag.ID {
				reviewItemID = item.ID
				break
			}
		}

		// First approve succeeds
		err = repo.ApproveReviewItem(ctx, reviewItemID, reviewerID, "OK")
		if err != nil {
			t.Fatalf("first approve failed: %v", err)
		}

		// Second approve should fail
		err = repo.ApproveReviewItem(ctx, reviewItemID, reviewerID, "Trying again")
		if err == nil {
			t.Error("second approve should fail because item is no longer pending")
		}
	})

	t.Run("AuditTrail_RecordsAllActions", func(t *testing.T) {
		// Create a tag, approve it, then verify the full lifecycle is in the audit trail
		req := models.CreateTagRequest{
			Name:    fmt.Sprintf("AuditTrail Tag %d", os.Getpid()),
			TagType: "skill",
		}
		tag, err := repo.CreateTag(ctx, req, createdBy)
		if err != nil {
			t.Fatalf("CreateTag failed: %v", err)
		}
		defer db.Exec(ctx, "DELETE FROM taxonomy_tags WHERE id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM taxonomy_review_queue WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)
		defer db.Exec(ctx, "DELETE FROM audit_log WHERE entity_type = 'tag' AND entity_id = $1", tag.ID)

		queue, _ := repo.GetReviewQueue(ctx)
		var reviewItemID int
		for _, item := range queue {
			if item.EntityType == "tag" && item.EntityID == tag.ID {
				reviewItemID = item.ID
				break
			}
		}
		if reviewItemID == 0 {
			t.Fatal("review queue entry not found")
		}

		// Approve it
		err = repo.ApproveReviewItem(ctx, reviewItemID, reviewerID, "Audit trail test")
		if err != nil {
			t.Fatalf("ApproveReviewItem failed: %v", err)
		}

		// Verify the audit trail has both submission and approval entries
		var submissionCount, approvalCount int
		db.QueryRow(ctx, `
			SELECT COUNT(*) FROM audit_log
			WHERE entity_type = 'tag' AND entity_id = $1 AND action = 'taxonomy_tag_submitted'
		`, tag.ID).Scan(&submissionCount)
		db.QueryRow(ctx, `
			SELECT COUNT(*) FROM audit_log
			WHERE entity_type = 'tag' AND entity_id = $1 AND action = 'taxonomy_review_approved'
		`, tag.ID).Scan(&approvalCount)

		if submissionCount == 0 {
			t.Error("audit trail missing submission entry")
		}
		if approvalCount == 0 {
			t.Error("audit trail missing approval entry")
		}

		// Verify GetReviewQueueAll returns the approved item
		allItems, err := repo.GetReviewQueueAll(ctx)
		if err != nil {
			t.Fatalf("GetReviewQueueAll failed: %v", err)
		}
		found := false
		for _, item := range allItems {
			if item.EntityType == "tag" && item.EntityID == tag.ID && item.Status == "approved" {
				found = true
				if item.ReviewedBy == nil || *item.ReviewedBy != reviewerID {
					t.Error("approved item should have reviewer_id set")
				}
				if item.DecisionNotes == nil || *item.DecisionNotes != "Audit trail test" {
					t.Error("approved item should have decision_notes set")
				}
			}
		}
		if !found {
			t.Error("approved item not found in GetReviewQueueAll audit trail")
		}
	})
}

// TestIntegration_TaxonomyReviewQueue_SynonymWorkflow tests the synonym creation -> review -> approve flow.
func TestIntegration_TaxonomyReviewQueue_SynonymWorkflow(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	repo := NewTaxonomyRepository(db)

	// Create a canonical tag first (directly with active status for testing)
	var canonicalTagID int
	err := db.QueryRow(ctx, `
		INSERT INTO taxonomy_tags (name, slug, tag_type, status)
		VALUES ('SynReview Canonical', 'synreview-canonical-'||floor(random()*100000)::int, 'skill', 'active')
		RETURNING id
	`).Scan(&canonicalTagID)
	if err != nil {
		t.Fatalf("create canonical tag: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM taxonomy_tags WHERE id = $1", canonicalTagID)

	createdBy := 1
	reviewerID := 1
	term := fmt.Sprintf("synreviewterm%d", os.Getpid())

	t.Run("CreateSynonym_StartsPendingReview", func(t *testing.T) {
		synonym, err := repo.CreateSynonym(ctx, term, canonicalTagID, createdBy)
		if err != nil {
			t.Fatalf("CreateSynonym failed: %v", err)
		}
		defer db.Exec(ctx, "DELETE FROM taxonomy_synonyms WHERE id = $1", synonym.ID)
		defer db.Exec(ctx, "DELETE FROM taxonomy_review_queue WHERE entity_type = 'synonym' AND entity_id = $1", synonym.ID)
		defer db.Exec(ctx, "DELETE FROM audit_log WHERE entity_type = 'synonym' AND entity_id = $1", synonym.ID)

		if synonym.Status != "pending_review" {
			t.Errorf("new synonym should have status 'pending_review', got %q", synonym.Status)
		}

		// Pending synonym should NOT be returned by FindSynonymMappings
		ids, err := repo.FindSynonymMappings(ctx, term)
		if err != nil {
			t.Fatalf("FindSynonymMappings failed: %v", err)
		}
		if len(ids) > 0 {
			t.Error("pending synonym should NOT be returned by FindSynonymMappings")
		}

		// Approve and verify it becomes active
		queue, _ := repo.GetReviewQueue(ctx)
		var reviewItemID int
		for _, item := range queue {
			if item.EntityType == "synonym" && item.EntityID == synonym.ID {
				reviewItemID = item.ID
				break
			}
		}
		if reviewItemID == 0 {
			t.Fatal("review queue entry not found for synonym")
		}

		err = repo.ApproveReviewItem(ctx, reviewItemID, reviewerID, "Approved synonym")
		if err != nil {
			t.Fatalf("approve synonym failed: %v", err)
		}

		// Now FindSynonymMappings should return it
		ids, err = repo.FindSynonymMappings(ctx, term)
		if err != nil {
			t.Fatalf("FindSynonymMappings after approve failed: %v", err)
		}
		if len(ids) == 0 {
			t.Error("approved synonym should be returned by FindSynonymMappings")
		}
	})
}
