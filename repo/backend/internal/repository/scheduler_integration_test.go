// Package repository contains integration tests for the scheduler run persistence.
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
	"testing"
)

// TestIntegration_SchedulerRepo_GetEnabledJobs verifies that seeded scheduled jobs are loaded.
func TestIntegration_SchedulerRepo_GetEnabledJobs(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	repo := NewSchedulerRepository(db)

	jobs, err := repo.GetEnabledJobs(ctx)
	if err != nil {
		t.Fatalf("GetEnabledJobs failed: %v", err)
	}

	if len(jobs) == 0 {
		t.Fatal("expected at least one enabled scheduled job from seed data")
	}

	// Verify expected seeded jobs exist
	expectedHandlers := map[string]bool{
		"recommendation_worker": false,
		"session_cleanup":       false,
		"archive_refresh":       false,
	}
	for _, job := range jobs {
		if _, ok := expectedHandlers[job.Handler]; ok {
			expectedHandlers[job.Handler] = true
		}
	}
	for handler, found := range expectedHandlers {
		if !found {
			t.Errorf("expected seeded job with handler %q not found", handler)
		}
	}
}

// TestIntegration_SchedulerRepo_RunLifecycle tests the full run lifecycle:
// start -> success, start -> failure, start -> compensation.
func TestIntegration_SchedulerRepo_RunLifecycle(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	repo := NewSchedulerRepository(db)

	// Get a job to work with
	jobs, err := repo.GetEnabledJobs(ctx)
	if err != nil || len(jobs) == 0 {
		t.Fatalf("need at least one enabled job: %v", err)
	}
	job := jobs[0]

	t.Run("SuccessfulRun", func(t *testing.T) {
		runID, err := repo.RecordRunStart(ctx, job.ID, 0)
		if err != nil {
			t.Fatalf("RecordRunStart failed: %v", err)
		}

		err = repo.RecordRunSuccess(ctx, runID, job.ID)
		if err != nil {
			t.Fatalf("RecordRunSuccess failed: %v", err)
		}

		// Verify run record
		var status string
		err = db.QueryRow(ctx, "SELECT status FROM scheduled_job_runs WHERE id = $1", runID).Scan(&status)
		if err != nil {
			t.Fatalf("query run status: %v", err)
		}
		if status != "success" {
			t.Errorf("expected run status 'success', got %q", status)
		}

		// Verify job was updated
		var lastStatus *string
		err = db.QueryRow(ctx, "SELECT last_status FROM scheduled_jobs WHERE id = $1", job.ID).Scan(&lastStatus)
		if err != nil || lastStatus == nil || *lastStatus != "success" {
			t.Errorf("expected job last_status 'success', got %v", lastStatus)
		}

		// Cleanup
		db.Exec(ctx, "DELETE FROM scheduled_job_runs WHERE id = $1", runID)
	})

	t.Run("FailedRun_IncrementsRetry", func(t *testing.T) {
		// Reset retry count
		repo.ResetRetryCount(ctx, job.ID)

		runID, err := repo.RecordRunStart(ctx, job.ID, 0)
		if err != nil {
			t.Fatalf("RecordRunStart failed: %v", err)
		}

		err = repo.RecordRunFailure(ctx, runID, job.ID, "test error: connection timeout")
		if err != nil {
			t.Fatalf("RecordRunFailure failed: %v", err)
		}

		// Verify run record
		var status string
		var errMsg *string
		err = db.QueryRow(ctx, "SELECT status, error_message FROM scheduled_job_runs WHERE id = $1", runID).Scan(&status, &errMsg)
		if err != nil {
			t.Fatalf("query run: %v", err)
		}
		if status != "failed" {
			t.Errorf("expected run status 'failed', got %q", status)
		}
		if errMsg == nil || *errMsg != "test error: connection timeout" {
			t.Errorf("expected error message 'test error: connection timeout', got %v", errMsg)
		}

		// Verify retry count incremented
		var retryCount int
		db.QueryRow(ctx, "SELECT retry_count FROM scheduled_jobs WHERE id = $1", job.ID).Scan(&retryCount)
		if retryCount != 1 {
			t.Errorf("expected retry_count=1, got %d", retryCount)
		}

		// Cleanup
		db.Exec(ctx, "DELETE FROM scheduled_job_runs WHERE id = $1", runID)
		repo.ResetRetryCount(ctx, job.ID)
	})

	t.Run("CompensatedRun_ResetsRetry", func(t *testing.T) {
		runID, err := repo.RecordRunStart(ctx, job.ID, 3)
		if err != nil {
			t.Fatalf("RecordRunStart failed: %v", err)
		}

		err = repo.RecordRunCompensated(ctx, runID, job.ID)
		if err != nil {
			t.Fatalf("RecordRunCompensated failed: %v", err)
		}

		// Verify run record
		var status string
		db.QueryRow(ctx, "SELECT status FROM scheduled_job_runs WHERE id = $1", runID).Scan(&status)
		if status != "compensated" {
			t.Errorf("expected run status 'compensated', got %q", status)
		}

		// Verify retry count was reset
		var retryCount int
		db.QueryRow(ctx, "SELECT retry_count FROM scheduled_jobs WHERE id = $1", job.ID).Scan(&retryCount)
		if retryCount != 0 {
			t.Errorf("expected retry_count=0 after compensation, got %d", retryCount)
		}

		// Cleanup
		db.Exec(ctx, "DELETE FROM scheduled_job_runs WHERE id = $1", runID)
	})

	t.Run("MultipleRetries_RecordsAll", func(t *testing.T) {
		repo.ResetRetryCount(ctx, job.ID)

		var runIDs []int64
		for attempt := 0; attempt < 3; attempt++ {
			runID, err := repo.RecordRunStart(ctx, job.ID, attempt)
			if err != nil {
				t.Fatalf("RecordRunStart attempt %d: %v", attempt, err)
			}
			runIDs = append(runIDs, runID)

			if attempt < 2 {
				repo.RecordRunFailure(ctx, runID, job.ID, "retry test")
			} else {
				repo.RecordRunSuccess(ctx, runID, job.ID)
			}
		}

		// Verify 3 run records exist
		var count int
		db.QueryRow(ctx, `
			SELECT COUNT(*) FROM scheduled_job_runs WHERE job_id = $1 AND id >= $2
		`, job.ID, runIDs[0]).Scan(&count)
		if count != 3 {
			t.Errorf("expected 3 run records, got %d", count)
		}

		// Cleanup
		for _, id := range runIDs {
			db.Exec(ctx, "DELETE FROM scheduled_job_runs WHERE id = $1", id)
		}
		repo.ResetRetryCount(ctx, job.ID)
	})
}
