// Package repository_test contains integration tests that run against a real PostgreSQL database.
// These tests require a running PostgreSQL instance with the schema from init.sql applied.
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

	"github.com/jackc/pgx/v5/pgxpool"
)

func getTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://wlpr:wlpr_secret@localhost:5432/wlpr_portal?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("failed to ping test DB: %v", err)
	}
	return pool
}

// TestIntegration_DisputeStateMachine_BlocksInvalidTransition tests that the PostgreSQL
// trigger trg_dispute_state_machine rejects a direct transition from CREATED to a
// resolved state, bypassing the required intermediate states.
func TestIntegration_DisputeStateMachine_BlocksInvalidTransition(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Create a temporary test vendor and review so we have valid FKs
	var vendorID, reviewID, disputeID int

	err := db.QueryRow(ctx, `
		INSERT INTO vendors (name, code, contact_email)
		VALUES ('Test Vendor IntDisp', 'INTDISP-' || floor(random()*100000)::int, 'test@intdisp.local')
		RETURNING id
	`).Scan(&vendorID)
	if err != nil {
		t.Fatalf("failed to create test vendor: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM vendors WHERE id = $1", vendorID)

	err = db.QueryRow(ctx, `
		INSERT INTO vendor_reviews (vendor_id, reviewer_id, rating, body)
		VALUES ($1, 1, 3, 'Integration test review for dispute state machine')
		RETURNING id
	`, vendorID).Scan(&reviewID)
	if err != nil {
		t.Fatalf("failed to create test review: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM vendor_reviews WHERE id = $1", reviewID)

	err = db.QueryRow(ctx, `
		INSERT INTO disputes (review_id, vendor_id, status, reason, created_by)
		VALUES ($1, $2, 'created', 'Integration test dispute', 1)
		RETURNING id
	`, reviewID, vendorID).Scan(&disputeID)
	if err != nil {
		t.Fatalf("failed to create test dispute: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM disputes WHERE id = $1", disputeID)

	// Verify the dispute starts in 'created' state
	var currentStatus string
	err = db.QueryRow(ctx, "SELECT status FROM disputes WHERE id = $1", disputeID).Scan(&currentStatus)
	if err != nil {
		t.Fatalf("failed to query dispute status: %v", err)
	}
	if currentStatus != "created" {
		t.Fatalf("expected initial status 'created', got %q", currentStatus)
	}

	// Attempt 1: Try to jump directly from 'created' to 'resolved_restored'
	// This MUST fail because the trigger only allows created -> evidence_uploaded
	_, err = db.Exec(ctx, "UPDATE disputes SET status = 'resolved_restored' WHERE id = $1", disputeID)
	if err == nil {
		t.Fatal("CRITICAL: Direct transition from 'created' to 'resolved_restored' should be BLOCKED by the trigger, but it succeeded")
	}
	t.Logf("Correctly blocked created->resolved_restored: %v", err)

	// Attempt 2: Try to jump from 'created' to 'arbitration' (skipping evidence_uploaded and under_review)
	_, err = db.Exec(ctx, "UPDATE disputes SET status = 'arbitration' WHERE id = $1", disputeID)
	if err == nil {
		t.Fatal("CRITICAL: Direct transition from 'created' to 'arbitration' should be BLOCKED by the trigger, but it succeeded")
	}
	t.Logf("Correctly blocked created->arbitration: %v", err)

	// Attempt 3: Try to jump from 'created' to 'under_review' (skipping evidence_uploaded)
	_, err = db.Exec(ctx, "UPDATE disputes SET status = 'under_review' WHERE id = $1", disputeID)
	if err == nil {
		t.Fatal("CRITICAL: Direct transition from 'created' to 'under_review' should be BLOCKED by the trigger, but it succeeded")
	}
	t.Logf("Correctly blocked created->under_review: %v", err)

	// Now perform the VALID transition path and verify each step
	// Step 1: created -> evidence_uploaded (VALID)
	_, err = db.Exec(ctx, "UPDATE disputes SET status = 'evidence_uploaded' WHERE id = $1", disputeID)
	if err != nil {
		t.Fatalf("valid transition created->evidence_uploaded should succeed: %v", err)
	}

	// Verify intermediate state from evidence_uploaded cannot skip
	_, err = db.Exec(ctx, "UPDATE disputes SET status = 'arbitration' WHERE id = $1", disputeID)
	if err == nil {
		t.Fatal("CRITICAL: Direct transition from 'evidence_uploaded' to 'arbitration' should be BLOCKED")
	}
	t.Logf("Correctly blocked evidence_uploaded->arbitration: %v", err)

	// Step 2: evidence_uploaded -> under_review (VALID)
	_, err = db.Exec(ctx, "UPDATE disputes SET status = 'under_review' WHERE id = $1", disputeID)
	if err != nil {
		t.Fatalf("valid transition evidence_uploaded->under_review should succeed: %v", err)
	}

	// Step 3: under_review -> arbitration (VALID)
	_, err = db.Exec(ctx, "UPDATE disputes SET status = 'arbitration' WHERE id = $1", disputeID)
	if err != nil {
		t.Fatalf("valid transition under_review->arbitration should succeed: %v", err)
	}

	// Step 4: arbitration -> resolved_restored (VALID)
	_, err = db.Exec(ctx, "UPDATE disputes SET status = 'resolved_restored' WHERE id = $1", disputeID)
	if err != nil {
		t.Fatalf("valid transition arbitration->resolved_restored should succeed: %v", err)
	}

	// Verify final state
	err = db.QueryRow(ctx, "SELECT status FROM disputes WHERE id = $1", disputeID).Scan(&currentStatus)
	if err != nil {
		t.Fatalf("failed to query final status: %v", err)
	}
	if currentStatus != "resolved_restored" {
		t.Fatalf("expected final status 'resolved_restored', got %q", currentStatus)
	}

	t.Log("All state machine transitions validated successfully")
}

// TestIntegration_DisputeStateMachine_ArbitrationOutcomes tests all valid arbitration outcomes.
func TestIntegration_DisputeStateMachine_ArbitrationOutcomes(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	outcomes := []string{"resolved_hidden", "resolved_disclaimer", "resolved_restored", "rejected"}

	for _, outcome := range outcomes {
		t.Run("arbitration->"+outcome, func(t *testing.T) {
			// Create fresh dispute for each outcome test
			var vendorID, reviewID, disputeID int

			code := fmt.Sprintf("ARBO-%s-%d", outcome[:4], os.Getpid())
			err := db.QueryRow(ctx, `
				INSERT INTO vendors (name, code) VALUES ('ArbitTest', $1) RETURNING id
			`, code).Scan(&vendorID)
			if err != nil {
				t.Fatalf("create vendor: %v", err)
			}
			defer db.Exec(ctx, "DELETE FROM vendors WHERE id = $1", vendorID)

			err = db.QueryRow(ctx, `
				INSERT INTO vendor_reviews (vendor_id, reviewer_id, rating, body)
				VALUES ($1, 1, 2, 'Test') RETURNING id
			`, vendorID).Scan(&reviewID)
			if err != nil {
				t.Fatalf("create review: %v", err)
			}
			defer db.Exec(ctx, "DELETE FROM vendor_reviews WHERE id = $1", reviewID)

			err = db.QueryRow(ctx, `
				INSERT INTO disputes (review_id, vendor_id, status, reason, created_by)
				VALUES ($1, $2, 'created', 'Test', 1) RETURNING id
			`, reviewID, vendorID).Scan(&disputeID)
			if err != nil {
				t.Fatalf("create dispute: %v", err)
			}
			defer db.Exec(ctx, "DELETE FROM disputes WHERE id = $1", disputeID)

			// Walk through valid path to arbitration
			for _, transition := range []string{"evidence_uploaded", "under_review", "arbitration"} {
				_, err = db.Exec(ctx, "UPDATE disputes SET status = $2 WHERE id = $1", disputeID, transition)
				if err != nil {
					t.Fatalf("transition to %s failed: %v", transition, err)
				}
			}

			// Apply the arbitration outcome
			_, err = db.Exec(ctx, "UPDATE disputes SET status = $2 WHERE id = $1", disputeID, outcome)
			if err != nil {
				t.Fatalf("arbitration -> %s should be valid: %v", outcome, err)
			}

			var finalStatus string
			db.QueryRow(ctx, "SELECT status FROM disputes WHERE id = $1", disputeID).Scan(&finalStatus)
			if finalStatus != outcome {
				t.Errorf("expected %q, got %q", outcome, finalStatus)
			}
		})
	}
}

// TestIntegration_SynonymConflict_BlockedByTrigger tests that the DB trigger
// check_synonym_conflict prevents two active synonyms for the same term
// pointing to different canonical tags.
func TestIntegration_SynonymConflict_BlockedByTrigger(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Create two test tags
	var tag1ID, tag2ID int
	err := db.QueryRow(ctx, `
		INSERT INTO taxonomy_tags (name, slug, tag_type) VALUES ('ConflictTestA', 'conflict-test-a-'||floor(random()*100000)::int, 'skill') RETURNING id
	`).Scan(&tag1ID)
	if err != nil {
		t.Fatalf("create tag1: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM taxonomy_tags WHERE id = $1", tag1ID)

	err = db.QueryRow(ctx, `
		INSERT INTO taxonomy_tags (name, slug, tag_type) VALUES ('ConflictTestB', 'conflict-test-b-'||floor(random()*100000)::int, 'skill') RETURNING id
	`).Scan(&tag2ID)
	if err != nil {
		t.Fatalf("create tag2: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM taxonomy_tags WHERE id = $1", tag2ID)

	uniqueTerm := fmt.Sprintf("testsynonym_%d", os.Getpid())

	// Insert first synonym: uniqueTerm -> tag1 (active)
	var synID int
	err = db.QueryRow(ctx, `
		INSERT INTO taxonomy_synonyms (term, canonical_tag_id, status)
		VALUES ($1, $2, 'active') RETURNING id
	`, uniqueTerm, tag1ID).Scan(&synID)
	if err != nil {
		t.Fatalf("first synonym insert failed: %v", err)
	}
	defer db.Exec(ctx, "DELETE FROM taxonomy_synonyms WHERE id = $1", synID)

	// Attempt second synonym: same term -> tag2 (active) — MUST fail
	_, err = db.Exec(ctx, `
		INSERT INTO taxonomy_synonyms (term, canonical_tag_id, status)
		VALUES ($1, $2, 'active')
	`, uniqueTerm, tag2ID)
	if err == nil {
		t.Fatal("CRITICAL: Inserting a second active synonym for the same term with a different canonical tag should be BLOCKED by the trigger")
	}
	t.Logf("Correctly blocked conflicting synonym: %v", err)
}
