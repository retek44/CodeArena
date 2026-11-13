// Package sweeper recovers submissions that would otherwise wait forever.
//
// The queue normally recovers a crashed worker by itself: the message becomes
// visible again and the next worker takes over the expired lease. That fails when
// the message is gone: after max_receive_count deliveries SQS moves it to the
// dead-letter queue, and the submission stays queued or running with nobody
// working on it. The sweeper finds such submissions in the database, which is the
// source of truth, and announces them again through the outbox (ADR-0011).
//
// Every sweep runs in one transaction with FOR UPDATE SKIP LOCKED, so several
// sweepers (one per API task) never handle the same submission twice.
package sweeper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/retek44/CodeArena/backend/internal/outbox"
	"github.com/retek44/CodeArena/backend/internal/store/db"
)

// Defaults, used when a Sweeper field is zero.
const (
	DefaultInterval     = time.Minute
	DefaultRunningAfter = 10 * time.Minute
	DefaultQueuedAfter  = 15 * time.Minute
	DefaultMaxAttempts  = 3
	DefaultBatchSize    = 100
)

// Sweeper requeues stuck submissions and fails the ones without attempts left.
type Sweeper struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	// Interval between sweeps.
	Interval time.Duration
	// RunningAfter: a running submission without progress for this long has lost
	// its worker. Must be well above the worker lease, so that the queue's own
	// redelivery gets the first chance.
	RunningAfter time.Duration
	// QueuedAfter: a queued submission nobody claimed for this long has lost its
	// message. Must be above the time workers need to scale out from zero.
	QueuedAfter time.Duration
	// MaxAttempts matches the worker's WORKER_MAX_ATTEMPTS: a running submission
	// with this many attempts gets IE instead of another try.
	MaxAttempts int
	// BatchSize limits the rows of one sweep; a full batch triggers the next sweep at once.
	BatchSize int
}

// Result lists what one sweep changed.
type Result struct {
	Requeued int
	Failed   int
}

// Run sweeps until ctx is cancelled.
func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(orDefault(s.Interval, DefaultInterval))
	defer ticker.Stop()

	for {
		res, err := s.SweepOnce(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			s.Logger.Error("sweep stuck submissions", "error", err)
		}
		// A full batch means more rows are probably waiting: continue at once.
		if err == nil && max(res.Requeued, res.Failed) >= s.batchSize() {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// SweepOnce handles one batch of stuck submissions.
func (s *Sweeper) SweepOnce(ctx context.Context) (Result, error) {
	runningAfter := interval(orDefault(s.RunningAfter, DefaultRunningAfter))
	maxAttempts := int64(s.MaxAttempts)
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := db.New(tx)

	// Fail first: a submission without attempts left must not be requeued.
	failed, err := q.FailExhaustedSubmissions(ctx, db.FailExhaustedSubmissionsParams{
		RunningAfter: runningAfter,
		MaxAttempts:  maxAttempts,
		MaxRows:      int32(s.batchSize()), //nolint:gosec // small positive batch size
	})
	if err != nil {
		return Result{}, fmt.Errorf("fail exhausted submissions: %w", err)
	}
	requeued, err := q.RequeueStuckSubmissions(ctx, db.RequeueStuckSubmissionsParams{
		RunningAfter: runningAfter,
		MaxAttempts:  maxAttempts,
		QueuedAfter:  interval(orDefault(s.QueuedAfter, DefaultQueuedAfter)),
		MaxRows:      int32(s.batchSize()), //nolint:gosec // small positive batch size
		Topic:        outbox.TopicSubmissionCreated,
	})
	if err != nil {
		return Result{}, fmt.Errorf("requeue stuck submissions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit: %w", err)
	}

	// One log line per submission: a metric filter or a log query can count them.
	for _, row := range failed {
		s.Logger.Warn("stuck submission failed", "submission_id", row.ID, "attempt", row.Attempt)
	}
	for _, row := range requeued {
		s.Logger.Warn("stuck submission requeued", "submission_id", row.ID, "attempt", row.Attempt)
	}
	return Result{Requeued: len(requeued), Failed: len(failed)}, nil
}

func (s *Sweeper) batchSize() int {
	if s.BatchSize > 0 {
		return s.BatchSize
	}
	return DefaultBatchSize
}

func orDefault(d, fallback time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return fallback
}

func interval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}
