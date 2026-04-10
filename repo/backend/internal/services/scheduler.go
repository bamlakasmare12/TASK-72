package services

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"wlpr-portal/internal/repository"
)

// JobHandler is a function that executes a scheduled job's logic.
// It returns an error if the job fails.
type JobHandler func(ctx context.Context) error

// CompensationHandler is called when a job exhausts all retries.
// It performs cleanup or rollback actions.
type CompensationHandler func(ctx context.Context, jobName string, lastErr error) error

// SchedulerService reads job definitions from the scheduled_jobs table,
// runs them on their cron schedules, persists run status/retries, and
// executes compensation strategies when retries are exhausted.
type SchedulerService struct {
	repo         *repository.SchedulerRepository
	handlers     map[string]JobHandler
	compensators map[string]CompensationHandler
	mu           sync.RWMutex
}

func NewSchedulerService(repo *repository.SchedulerRepository) *SchedulerService {
	return &SchedulerService{
		repo:         repo,
		handlers:     make(map[string]JobHandler),
		compensators: make(map[string]CompensationHandler),
	}
}

// RegisterHandler registers a job handler function for a given handler name.
func (s *SchedulerService) RegisterHandler(name string, handler JobHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[name] = handler
}

// RegisterCompensation registers a compensation handler for a given handler name.
func (s *SchedulerService) RegisterCompensation(name string, handler CompensationHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compensators[name] = handler
}

// Start launches the scheduler loop. It polls the DB for enabled jobs every pollInterval,
// checks if each job is due based on its cron expression, and executes due jobs.
func (s *SchedulerService) Start(ctx context.Context, pollInterval time.Duration) {
	go func() {
		log.Printf("[scheduler] started with poll interval %s", pollInterval)

		// Run initial check immediately
		s.tick(ctx)

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[scheduler] shutting down")
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

func (s *SchedulerService) tick(ctx context.Context) {
	jobs, err := s.repo.GetEnabledJobs(ctx)
	if err != nil {
		log.Printf("[scheduler] failed to load jobs: %v", err)
		return
	}

	now := time.Now()
	for _, job := range jobs {
		if !s.isDue(job, now) {
			continue
		}

		s.mu.RLock()
		handler, ok := s.handlers[job.Handler]
		s.mu.RUnlock()

		if !ok {
			log.Printf("[scheduler] no handler registered for '%s' (job: %s)", job.Handler, job.Name)
			continue
		}

		go s.executeJob(ctx, job, handler)
	}
}

// isDue checks if a job should run based on its cron expression and last_run_at.
// Supports simplified cron: "*/N * * * *" (every N minutes), "0 H * * *" (daily at hour H).
func (s *SchedulerService) isDue(job repository.ScheduledJob, now time.Time) bool {
	if job.LastRunAt == nil {
		return true // Never run before
	}

	interval := ParseCronInterval(job.CronExpr)
	if interval == 0 {
		return false
	}

	return now.Sub(*job.LastRunAt) >= interval
}

// ParseCronInterval converts a cron expression to a duration.
// Supports:
//   - "*/N * * * *" -> every N minutes
//   - "0 H * * *"   -> every 24 hours (daily jobs)
//   - "0 H * * D"   -> every 7 days (weekly jobs)
func ParseCronInterval(expr string) time.Duration {
	parts := strings.Fields(expr)
	if len(parts) < 5 {
		return 0
	}

	minute := parts[0]
	// Every-N-minutes pattern
	if strings.HasPrefix(minute, "*/") {
		n, err := strconv.Atoi(strings.TrimPrefix(minute, "*/"))
		if err != nil || n <= 0 {
			return 0
		}
		return time.Duration(n) * time.Minute
	}

	// Fixed-hour daily pattern: "0 H * * *"
	if parts[2] == "*" && parts[3] == "*" && parts[4] == "*" {
		return 24 * time.Hour
	}

	// Fixed-hour weekly pattern: "0 H * * D" (D is specific day)
	if parts[2] == "*" && parts[3] == "*" && parts[4] != "*" {
		return 7 * 24 * time.Hour
	}

	// Default: treat as daily
	return 24 * time.Hour
}

// executeJob runs a job handler with retry support.
// On failure, retries up to MaxRetries. If all retries fail, runs compensation.
func (s *SchedulerService) executeJob(ctx context.Context, job repository.ScheduledJob, handler JobHandler) {
	retryAttempt := job.RetryCount

	for attempt := retryAttempt; attempt <= job.MaxRetries; attempt++ {
		runID, err := s.repo.RecordRunStart(ctx, job.ID, attempt)
		if err != nil {
			log.Printf("[scheduler] failed to record run start for job '%s': %v", job.Name, err)
			return
		}

		log.Printf("[scheduler] executing job '%s' (attempt %d/%d)", job.Name, attempt+1, job.MaxRetries+1)

		jobErr := handler(ctx)
		if jobErr == nil {
			// Success
			if err := s.repo.RecordRunSuccess(ctx, runID, job.ID); err != nil {
				log.Printf("[scheduler] failed to record success for job '%s': %v", job.Name, err)
			}
			log.Printf("[scheduler] job '%s' completed successfully", job.Name)
			return
		}

		// Failure
		log.Printf("[scheduler] job '%s' failed (attempt %d/%d): %v", job.Name, attempt+1, job.MaxRetries+1, jobErr)
		if err := s.repo.RecordRunFailure(ctx, runID, job.ID, jobErr.Error()); err != nil {
			log.Printf("[scheduler] failed to record failure for job '%s': %v", job.Name, err)
		}

		if attempt >= job.MaxRetries {
			// All retries exhausted: run compensation
			s.runCompensation(ctx, job, runID, jobErr)
			return
		}

		// Wait before retry (exponential backoff: 30s, 60s, 120s, ...)
		backoff := time.Duration(1<<uint(attempt)) * 30 * time.Second
		log.Printf("[scheduler] job '%s' retrying in %s", job.Name, backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

// runCompensation executes the compensation handler for a failed job.
func (s *SchedulerService) runCompensation(ctx context.Context, job repository.ScheduledJob, runID int64, lastErr error) {
	s.mu.RLock()
	compensator, ok := s.compensators[job.Handler]
	s.mu.RUnlock()

	if !ok {
		log.Printf("[scheduler] no compensation handler for job '%s'; marking as compensated (noop)", job.Name)
		_ = s.repo.RecordRunCompensated(ctx, runID, job.ID)
		return
	}

	log.Printf("[scheduler] running compensation for job '%s'", job.Name)
	if err := compensator(ctx, job.Name, lastErr); err != nil {
		log.Printf("[scheduler] compensation failed for job '%s': %v", job.Name, err)
	} else {
		log.Printf("[scheduler] compensation completed for job '%s'", job.Name)
	}
	_ = s.repo.RecordRunCompensated(ctx, runID, job.ID)
}

// RunJobByName manually triggers a job by its handler name (for testing or admin use).
func (s *SchedulerService) RunJobByName(ctx context.Context, handlerName string) error {
	s.mu.RLock()
	handler, ok := s.handlers[handlerName]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler registered for '%s'", handlerName)
	}

	return handler(ctx)
}
