// Package outbox implements the transactional outbox (ADR-0011): messages are stored
// in the same database transaction as the change they announce, and a relay
// publishes them to the queue afterwards (at-least-once).
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/retek44/CodeArena/backend/internal/store/db"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/telemetry"
)

// TopicSubmissionCreated announces a new submission to the judge workers.
const TopicSubmissionCreated = "submission.created"

// SubmissionCreated is the payload of TopicSubmissionCreated. The worker loads
// everything else from the database, so the message stays small and never stale.
type SubmissionCreated struct {
	Version      int       `json:"v"`
	SubmissionID uuid.UUID `json:"submission_id"`
}

// AddSubmissionCreated stores the message in the caller's transaction.
func AddSubmissionCreated(ctx context.Context, tx *ent.Tx, id uuid.UUID) error {
	payload, err := json.Marshal(SubmissionCreated{Version: 1, SubmissionID: id})
	if err != nil {
		return err
	}
	telemetry.TagSubmission(ctx, id.String())
	return tx.OutboxMessage.Create().
		SetTopic(TopicSubmissionCreated).
		SetPayload(payload).
		SetTraceParent(traceParent(ctx)).
		Exec(ctx)
}

// traceParent returns the W3C traceparent of the context's span ("" without one).
func traceParent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	return carrier.Get("traceparent")
}

// Message is one pending outbox row.
type Message struct {
	ID      int64
	Topic   string
	Payload []byte
}

// Publisher sends a message to the queue of its topic.
type Publisher interface {
	Publish(ctx context.Context, m Message) error
}

// Relay publishes pending messages. Several relays can run at once (one per API
// task): rows are locked with FOR UPDATE SKIP LOCKED.
//
// Delivery is at-least-once: if the commit fails after a publish, the message is
// sent again later. Consumers must be idempotent (the worker's claim is).
type Relay struct {
	Pool      *pgxpool.Pool
	Publisher Publisher
	Logger    *slog.Logger
	BatchSize int
	Interval  time.Duration
}

func (r *Relay) batchSize() int {
	if r.BatchSize > 0 {
		return r.BatchSize
	}
	return 10
}

// Run relays until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) {
	interval := r.Interval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		sent, err := r.RelayOnce(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			r.Logger.Error("outbox relay", "error", err)
		}
		// A full batch means more rows are probably waiting: continue at once.
		if sent == r.batchSize() {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RelayOnce publishes one batch and returns the number of messages sent.
func (r *Relay) RelayOnce(ctx context.Context) (int, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	q := db.New(tx)
	rows, err := q.LockPendingOutbox(ctx, int32(r.batchSize())) //nolint:gosec // small positive batch size
	if err != nil {
		return 0, fmt.Errorf("lock pending messages: %w", err)
	}

	var sent []int64
	for _, row := range rows {
		err := r.publish(ctx, row)
		if err == nil {
			sent = append(sent, row.ID)
			continue
		}
		r.Logger.Warn("publish outbox message", "id", row.ID, "topic", row.Topic, "attempt", row.Attempts+1, "error", err)
		if err := q.MarkOutboxFailed(ctx, db.MarkOutboxFailedParams{
			ID:        row.ID,
			LastError: err.Error(),
			Backoff:   interval(Backoff(int(row.Attempts) + 1)),
		}); err != nil {
			return 0, fmt.Errorf("mark message %d failed: %w", row.ID, err)
		}
	}

	if len(sent) > 0 {
		if err := q.MarkOutboxSent(ctx, sent); err != nil {
			return 0, fmt.Errorf("mark sent: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return len(sent), nil
}

// publish sends one row inside the trace of the request that wrote it.
func (r *Relay) publish(ctx context.Context, row db.LockPendingOutboxRow) error {
	parent := propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{"traceparent": row.TraceParent.String})
	ctx, span := telemetry.Tracer().Start(parent, row.Topic+" send",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(semconv.MessagingOperationTypeSend, semconv.MessagingDestinationName(row.Topic)))
	defer span.End()

	err := r.Publisher.Publish(ctx, Message{ID: row.ID, Topic: row.Topic, Payload: row.Payload})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
	}
	return err
}

// Backoff is the delay before the next publish attempt: 2^attempt seconds, at most 5 minutes.
func Backoff(attempt int) time.Duration {
	if attempt > 8 {
		return 5 * time.Minute
	}
	return min(time.Duration(1<<attempt)*time.Second, 5*time.Minute)
}

func interval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}

// compile-time check: pgx.Tx satisfies the sqlc DBTX interface.
var _ db.DBTX = pgx.Tx(nil)
