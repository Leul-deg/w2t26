// Package scheduler provides an in-process nightly job runner.
// Designed for offline-local deployment: no cron daemon, no external message
// queue. A goroutine wakes at midnight (local time) and runs registered jobs.
// Jobs are also triggerable on-demand via the reports recalculate endpoint.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Job is a function that the scheduler calls at midnight each day.
type Job struct {
	Name string
	Run  func(ctx context.Context, date time.Time) error
}

// Scheduler fires registered jobs once per day at midnight local time.
type Scheduler struct {
	jobs []Job
}

// New creates a Scheduler with the given jobs.
func New(jobs ...Job) *Scheduler {
	return &Scheduler{jobs: jobs}
}

// Start begins the nightly loop. It blocks until ctx is cancelled.
// Call it in a goroutine: go scheduler.Start(ctx).
func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("nightly scheduler started", "job_count", len(s.jobs))
	for {
		next := nextMidnight()
		slog.Info("nightly scheduler sleeping", "next_run", next.Format(time.RFC3339), "until", time.Until(next).Round(time.Minute))

		select {
		case <-ctx.Done():
			slog.Info("nightly scheduler stopped")
			return
		case <-time.After(time.Until(next)):
			s.runAll(ctx, next)
		}
	}
}

// runAll executes all jobs for the given date.
func (s *Scheduler) runAll(ctx context.Context, date time.Time) {
	for _, j := range s.jobs {
		slog.Info("nightly job starting", "job", j.Name, "date", date.Format("2006-01-02"))
		if err := j.Run(ctx, date); err != nil {
			slog.Error("nightly job failed", "job", j.Name, "error", err)
		} else {
			slog.Info("nightly job completed", "job", j.Name)
		}
	}
}

// nextMidnight returns the next midnight in local time.
func nextMidnight() time.Time {
	now := time.Now()
	y, m, d := now.Date()
	loc := now.Location()
	midnight := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
	return midnight
}
