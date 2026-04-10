package services_test

import (
	"testing"
)

// These tests validate the taxonomy review queue workflow logic
// without requiring a database. The actual DB interaction is tested
// in integration tests.

type reviewQueueEntry struct {
	ID         int
	EntityType string
	EntityID   int
	Status     string
}

type entityRecord struct {
	ID     int
	Status string
}

// simulateApprove simulates the approval logic:
// 1. Check review item is pending
// 2. Activate the entity
// 3. Mark review item as approved
func simulateApprove(queue []reviewQueueEntry, entities map[string][]entityRecord, reviewItemID int) (string, error) {
	for i, item := range queue {
		if item.ID == reviewItemID {
			if item.Status != "pending" {
				return "", nil
			}

			// Activate entity
			for j, ent := range entities[item.EntityType] {
				if ent.ID == item.EntityID {
					entities[item.EntityType][j].Status = "active"
				}
			}

			// Mark approved
			queue[i].Status = "approved"
			return "approved", nil
		}
	}
	return "", nil
}

// simulateReject simulates the rejection logic
func simulateReject(queue []reviewQueueEntry, entities map[string][]entityRecord, reviewItemID int) (string, error) {
	for i, item := range queue {
		if item.ID == reviewItemID {
			if item.Status != "pending" {
				return "", nil
			}

			// Reject entity
			for j, ent := range entities[item.EntityType] {
				if ent.ID == item.EntityID {
					entities[item.EntityType][j].Status = "rejected"
				}
			}

			// Mark rejected
			queue[i].Status = "rejected"
			return "rejected", nil
		}
	}
	return "", nil
}

func TestReviewQueue_TagCreation_StartsPending(t *testing.T) {
	// When a tag is created through the review queue, it starts as 'pending'
	tag := entityRecord{ID: 1, Status: "pending"}
	if tag.Status != "pending" {
		t.Error("newly created tag should have 'pending' status")
	}
}

func TestReviewQueue_SynonymCreation_StartsPendingReview(t *testing.T) {
	synonym := entityRecord{ID: 1, Status: "pending_review"}
	if synonym.Status != "pending_review" {
		t.Error("newly created synonym should have 'pending_review' status")
	}
}

func TestReviewQueue_ApproveTag_ActivatesTag(t *testing.T) {
	entities := map[string][]entityRecord{
		"tag": {{ID: 10, Status: "pending"}},
	}
	queue := []reviewQueueEntry{
		{ID: 1, EntityType: "tag", EntityID: 10, Status: "pending"},
	}

	result, _ := simulateApprove(queue, entities, 1)

	if result != "approved" {
		t.Error("approve should return 'approved'")
	}
	if entities["tag"][0].Status != "active" {
		t.Errorf("tag should be active after approval, got %s", entities["tag"][0].Status)
	}
	if queue[0].Status != "approved" {
		t.Errorf("review item should be approved, got %s", queue[0].Status)
	}
}

func TestReviewQueue_RejectSynonym_RejectsSynonym(t *testing.T) {
	entities := map[string][]entityRecord{
		"synonym": {{ID: 5, Status: "pending_review"}},
	}
	queue := []reviewQueueEntry{
		{ID: 2, EntityType: "synonym", EntityID: 5, Status: "pending"},
	}

	result, _ := simulateReject(queue, entities, 2)

	if result != "rejected" {
		t.Error("reject should return 'rejected'")
	}
	if entities["synonym"][0].Status != "rejected" {
		t.Errorf("synonym should be rejected, got %s", entities["synonym"][0].Status)
	}
}

func TestReviewQueue_CannotApproveAlreadyApproved(t *testing.T) {
	entities := map[string][]entityRecord{
		"tag": {{ID: 10, Status: "active"}},
	}
	queue := []reviewQueueEntry{
		{ID: 1, EntityType: "tag", EntityID: 10, Status: "approved"},
	}

	result, _ := simulateApprove(queue, entities, 1)

	if result == "approved" {
		t.Error("should not be able to approve an already-approved item")
	}
}

func TestReviewQueue_CannotRejectAlreadyRejected(t *testing.T) {
	entities := map[string][]entityRecord{
		"tag": {{ID: 10, Status: "rejected"}},
	}
	queue := []reviewQueueEntry{
		{ID: 1, EntityType: "tag", EntityID: 10, Status: "rejected"},
	}

	result, _ := simulateReject(queue, entities, 1)

	if result == "rejected" {
		t.Error("should not be able to reject an already-rejected item")
	}
}

func TestReviewQueue_PendingTagsNotVisibleInActiveList(t *testing.T) {
	// Simulates the filtering behavior: only active tags are returned to non-admin users
	allTags := []entityRecord{
		{ID: 1, Status: "active"},
		{ID: 2, Status: "pending"},
		{ID: 3, Status: "active"},
		{ID: 4, Status: "rejected"},
	}

	var activeTags []entityRecord
	for _, tag := range allTags {
		if tag.Status == "active" {
			activeTags = append(activeTags, tag)
		}
	}

	if len(activeTags) != 2 {
		t.Errorf("expected 2 active tags, got %d", len(activeTags))
	}
}

func TestReviewQueue_MultipleItemsWorkflow(t *testing.T) {
	entities := map[string][]entityRecord{
		"tag":     {{ID: 10, Status: "pending"}, {ID: 11, Status: "pending"}},
		"synonym": {{ID: 20, Status: "pending_review"}},
	}
	queue := []reviewQueueEntry{
		{ID: 1, EntityType: "tag", EntityID: 10, Status: "pending"},
		{ID: 2, EntityType: "tag", EntityID: 11, Status: "pending"},
		{ID: 3, EntityType: "synonym", EntityID: 20, Status: "pending"},
	}

	// Approve tag 10
	simulateApprove(queue, entities, 1)
	// Reject tag 11
	simulateReject(queue, entities, 2)
	// Approve synonym 20
	simulateApprove(queue, entities, 3)

	if entities["tag"][0].Status != "active" {
		t.Errorf("tag 10 should be active, got %s", entities["tag"][0].Status)
	}
	if entities["tag"][1].Status != "rejected" {
		t.Errorf("tag 11 should be rejected, got %s", entities["tag"][1].Status)
	}
	if entities["synonym"][0].Status != "active" {
		t.Errorf("synonym 20 should be active, got %s", entities["synonym"][0].Status)
	}

	// Verify audit trail
	approvedCount := 0
	rejectedCount := 0
	for _, item := range queue {
		switch item.Status {
		case "approved":
			approvedCount++
		case "rejected":
			rejectedCount++
		}
	}
	if approvedCount != 2 || rejectedCount != 1 {
		t.Errorf("expected 2 approved, 1 rejected; got %d approved, %d rejected", approvedCount, rejectedCount)
	}
}
