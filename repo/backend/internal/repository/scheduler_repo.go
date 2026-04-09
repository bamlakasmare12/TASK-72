package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ScheduledJob represents a job definition from the scheduled_jobs table.
type ScheduledJob struct {
	ID         int
	Name       string
	CronExpr   string
	Handler    string
	Enabled    bool
	LastRunAt  *time.Time
	LastStatus *string
	RetryCount int
	MaxRetries int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ScheduledJobRun represents a single execution record of a scheduled job.
type ScheduledJobRun struct {
	ID           int64
	JobID        int
	Status       string
	StartedAt    time.Time
	FinishedAt   *time.Time
	ErrorMessage *string
	RetryAttempt int
}

type SchedulerRepository struct {
	db *pgxpool.Pool
}

func NewSchedulerRepository(db *pgxpool.Pool) *SchedulerRepository {
	return &SchedulerRepository{db: db}
}

// GetEnabledJobs returns all enabled scheduled jobs.
func (r *SchedulerRepository) GetEnabledJobs(ctx context.Context) ([]ScheduledJob, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, cron_expr, handler, enabled,
			   last_run_at, last_status, retry_count, max_retries,
			   created_at, updated_at
		FROM scheduled_jobs
		WHERE enabled = TRUE
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]ScheduledJob, 0)
	for rows.Next() {
		var j ScheduledJob
		if err := rows.Scan(&j.ID, &j.Name, &j.CronExpr, &j.Handler, &j.Enabled,
			&j.LastRunAt, &j.LastStatus, &j.RetryCount, &j.MaxRetries,
			&j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// RecordRunStart inserts a new job run record with status 'running'.
func (r *SchedulerRepository) RecordRunStart(ctx context.Context, jobID, retryAttempt int) (int64, error) {
	var runID int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO scheduled_job_runs (job_id, status, started_at, retry_attempt)
		VALUES ($1, 'running', NOW(), $2)
		RETURNING id
	`, jobID, retryAttempt).Scan(&runID)
	return runID, err
}

// RecordRunSuccess marks a job run as successful and updates the parent job.
func (r *SchedulerRepository) RecordRunSuccess(ctx context.Context, runID int64, jobID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE scheduled_job_runs
		SET status = 'success', finished_at = NOW()
		WHERE id = $1
	`, runID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		UPDATE scheduled_jobs
		SET last_run_at = NOW(), last_status = 'success', retry_count = 0, updated_at = NOW()
		WHERE id = $1
	`, jobID)
	return err
}

// RecordRunFailure marks a job run as failed, increments retry count, and stores the error.
func (r *SchedulerRepository) RecordRunFailure(ctx context.Context, runID int64, jobID int, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE scheduled_job_runs
		SET status = 'failed', finished_at = NOW(), error_message = $2
		WHERE id = $1
	`, runID, errMsg)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		UPDATE scheduled_jobs
		SET last_run_at = NOW(), last_status = 'failed',
			retry_count = retry_count + 1, updated_at = NOW()
		WHERE id = $1
	`, jobID)
	return err
}

// RecordRunCompensated marks a job run as compensated (retry exhausted, compensation applied).
func (r *SchedulerRepository) RecordRunCompensated(ctx context.Context, runID int64, jobID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE scheduled_job_runs
		SET status = 'compensated', finished_at = NOW()
		WHERE id = $1
	`, runID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		UPDATE scheduled_jobs
		SET last_run_at = NOW(), last_status = 'compensated',
			retry_count = 0, updated_at = NOW()
		WHERE id = $1
	`, jobID)
	return err
}

// ResetRetryCount resets the retry count for a job (used after successful compensation).
func (r *SchedulerRepository) ResetRetryCount(ctx context.Context, jobID int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE scheduled_jobs SET retry_count = 0, updated_at = NOW() WHERE id = $1
	`, jobID)
	return err
}
