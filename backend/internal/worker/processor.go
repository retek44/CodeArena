// Package worker judges queued submissions (phase 3 of docs/PLAN.md).
//
// Flow for one message: claim the submission with a lease (sqlc ClaimSubmission),
// load the test cases of its problem version, run them on the runner, then store
// the per-case results and the verdict in one transaction.
//
// Every step is safe to repeat: SQS may deliver a message twice, and a worker may
// crash at any point. A crashed worker's lease expires and the next delivery takes over.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/retek44/CodeArena/backend/internal/judge"
	"github.com/retek44/CodeArena/backend/internal/outbox"
	"github.com/retek44/CodeArena/backend/internal/problem"
	"github.com/retek44/CodeArena/backend/internal/queue"
	"github.com/retek44/CodeArena/backend/internal/runner"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/db"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	entproblem "github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
	"github.com/retek44/CodeArena/backend/internal/store/ent/testcase"
)

// Outcome describes what happened to a submission.
type Outcome string

// Outcomes.
const (
	// Judged: the verdict is stored.
	Judged Outcome = "judged"
	// Finished: the submission was already done or failed (duplicate delivery).
	Finished Outcome = "finished"
	// Busy: another worker holds a valid lease; try again later.
	Busy Outcome = "busy"
	// Requeued: judging failed (runner error); the submission waits for the next attempt.
	Requeued Outcome = "requeued"
	// Failed: the retry budget is used up; the verdict is IE (internal error).
	Failed Outcome = "failed"
	// Dropped: the message or the submission is invalid; retrying cannot help.
	Dropped Outcome = "dropped"
)

// Processor judges submissions.
type Processor struct {
	Store   *store.Store
	Invoker runner.Invoker
	Logger  *slog.Logger
	// Lease must be longer than the slowest judging (the runner Lambda timeout) and
	// match the queue's visibility timeout.
	Lease time.Duration
	// MaxAttempts is the number of judging attempts before the verdict becomes IE.
	MaxAttempts int
}

// Handle implements queue.Handler.
func (p *Processor) Handle(ctx context.Context, d queue.Delivery) queue.Decision {
	var msg outbox.SubmissionCreated
	if err := json.Unmarshal(d.Body, &msg); err != nil || msg.Version != 1 || msg.SubmissionID == uuid.Nil {
		p.Logger.Error("drop malformed message", "body", truncate(string(d.Body), 200), "error", err)
		return queue.Ack
	}

	start := time.Now()
	outcome, err := p.Process(ctx, msg.SubmissionID)
	log := p.Logger.With("submission_id", msg.SubmissionID, "outcome", outcome,
		"receive_count", d.ReceiveCount, "duration_ms", time.Since(start).Milliseconds())
	if err != nil {
		log.Error("process submission", "error", err)
		return queue.Retry
	}
	log.Info("processed submission")

	switch outcome {
	case Busy, Requeued:
		return queue.Retry
	default:
		return queue.Ack
	}
}

// Process judges one submission. An error means a temporary problem (database);
// the message should be retried.
func (p *Processor) Process(ctx context.Context, id uuid.UUID) (Outcome, error) {
	q := p.Store.Queries
	claimed, err := q.ClaimSubmission(ctx, db.ClaimSubmissionParams{ID: id, Lease: interval(p.Lease)})
	if errors.Is(err, pgx.ErrNoRows) {
		return p.notClaimed(ctx, id)
	}
	if err != nil {
		return "", fmt.Errorf("claim: %w", err)
	}

	prob, err := p.loadProblem(ctx, int(claimed.ProblemID), int(claimed.ProblemVersion))
	if err != nil {
		// Broken problem data cannot be fixed by retrying.
		p.Logger.Error("load problem", "submission_id", id, "error", err)
		return p.fail(ctx, id)
	}

	result, err := judge.Run(ctx, p.Invoker, prob, judge.Submission{
		ID:       id.String(),
		Language: claimed.Language,
		Source:   claimed.Source,
	})
	if err == nil && result.Verdict == judge.InternalError {
		err = errors.New("runner skipped test cases (time budget)")
	}
	if err != nil {
		p.Logger.Warn("judging attempt failed", "submission_id", id, "attempt", claimed.Attempt, "error", err)
		if int(claimed.Attempt) >= p.MaxAttempts {
			return p.fail(ctx, id)
		}
		if _, err := q.RequeueSubmission(ctx, id); err != nil {
			return "", fmt.Errorf("requeue: %w", err)
		}
		return Requeued, nil
	}

	if err := p.store(ctx, id, result); err != nil {
		return "", err
	}
	return Judged, nil
}

// notClaimed decides what a failed claim means.
func (p *Processor) notClaimed(ctx context.Context, id uuid.UUID) (Outcome, error) {
	sub, err := p.Store.Ent.Submission.Query().Where(submission.ID(id)).Select(submission.FieldStatus).Only(ctx)
	if ent.IsNotFound(err) {
		return Dropped, nil
	}
	if err != nil {
		return "", fmt.Errorf("read status: %w", err)
	}
	if sub.Status == submission.StatusRunning {
		// Another worker is judging it. If that worker dies, its lease expires and
		// the redelivered message takes over.
		return Busy, nil
	}
	return Finished, nil
}

func (p *Processor) fail(ctx context.Context, id uuid.UUID) (Outcome, error) {
	if _, err := p.Store.Queries.FailSubmission(ctx, id); err != nil {
		return "", fmt.Errorf("fail submission: %w", err)
	}
	return Failed, nil
}

func (p *Processor) loadProblem(ctx context.Context, problemID, version int) (*problem.Problem, error) {
	prob, err := p.Store.Ent.Problem.Query().Where(entproblem.ID(problemID)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("problem %d: %w", problemID, err)
	}
	cases, err := prob.QueryTestCases().
		Where(testcase.ProblemVersion(version)).
		Order(ent.Asc(testcase.FieldOrdinal)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("test cases: %w", err)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("problem %s version %d has no test cases", prob.Slug, version)
	}

	out := &problem.Problem{Slug: prob.Slug, TimeMS: prob.TimeLimitMs, MemoryMB: prob.MemoryLimitMB, Checker: string(prob.Checker)}
	for _, tc := range cases {
		out.Cases = append(out.Cases, problem.Case{Name: strconv.Itoa(tc.Ordinal), Input: tc.Input, Expected: tc.ExpectedOutput})
	}
	return out, nil
}

// store writes the per-case results and the verdict atomically.
func (p *Processor) store(ctx context.Context, id uuid.UUID, r judge.Result) error {
	params := db.InsertSubmissionResultsParams{SubmissionID: id}
	var runtime, memory int64
	for _, c := range r.Cases {
		ordinal, err := strconv.ParseInt(c.Name, 10, 64)
		if err != nil {
			return fmt.Errorf("case name %q is not an ordinal: %w", c.Name, err)
		}
		params.Ordinals = append(params.Ordinals, ordinal)
		params.Verdicts = append(params.Verdicts, string(c.Verdict))
		params.RuntimesMs = append(params.RuntimesMs, int64(c.CPUTimeMS))
		params.MemoriesKb = append(params.MemoriesKb, int64(c.MemoryKB))
		runtime = max(runtime, int64(c.CPUTimeMS))
		memory = max(memory, int64(c.MemoryKB))
	}

	complete := db.CompleteSubmissionParams{ID: id, Verdict: string(r.Verdict)}
	if r.Verdict == judge.CompileError {
		complete.CompileOutput = pgtype.Text{String: r.CompileOutput, Valid: true}
	} else {
		complete.RuntimeMs = pgtype.Int8{Int64: runtime, Valid: true}
		complete.MemoryKb = pgtype.Int8{Int64: memory, Valid: true}
	}

	tx, err := p.Store.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	qtx := p.Store.Queries.WithTx(tx)
	if len(params.Ordinals) > 0 {
		if err := qtx.InsertSubmissionResults(ctx, params); err != nil {
			return fmt.Errorf("insert results: %w", err)
		}
	}
	rows, err := qtx.CompleteSubmission(ctx, complete)
	if err != nil {
		return fmt.Errorf("complete: %w", err)
	}
	if rows == 0 {
		// The lease expired and another worker finished first; keep its result.
		return nil
	}
	return tx.Commit(ctx)
}

func interval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
