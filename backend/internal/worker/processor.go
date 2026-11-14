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

	"github.com/retek44/CodeArena/backend/internal/contest"
	"github.com/retek44/CodeArena/backend/internal/events"
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
	"github.com/retek44/CodeArena/backend/internal/telemetry"
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
	// Events announces status changes (WebSocket push); nil publishes nothing.
	Events events.Publisher
	// Leaderboard is updated after contest submissions; nil skips the cache update.
	Leaderboard *contest.Leaderboard
	// Now is the clock (tests); nil means time.Now.
	Now func() time.Time
}

// Handle implements queue.Handler.
func (p *Processor) Handle(ctx context.Context, d queue.Delivery) queue.Decision {
	var msg outbox.SubmissionCreated
	if err := json.Unmarshal(d.Body, &msg); err != nil || msg.Version != 1 || msg.SubmissionID == uuid.Nil {
		p.Logger.ErrorContext(ctx, "drop malformed message", "body", truncate(string(d.Body), 200), "error", err)
		return queue.Ack
	}

	telemetry.TagSubmission(ctx, msg.SubmissionID.String())
	start := time.Now()
	outcome, err := p.Process(ctx, msg.SubmissionID)
	log := p.Logger.With("submission_id", msg.SubmissionID, "outcome", outcome,
		"receive_count", d.ReceiveCount, "duration_ms", time.Since(start).Milliseconds())
	if err != nil {
		log.ErrorContext(ctx, "process submission", "error", err)
		return queue.Retry
	}
	log.InfoContext(ctx, "processed submission")

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
	p.publish(ctx, events.SubmissionUpdated{SubmissionID: id, UserID: claimed.UserID, Status: "running"})

	prob, err := p.loadProblem(ctx, int(claimed.ProblemID), int(claimed.ProblemVersion))
	if err != nil {
		// Broken problem data cannot be fixed by retrying.
		p.Logger.ErrorContext(ctx, "load problem", "submission_id", id, "error", err)
		return p.fail(ctx, id, claimed.UserID)
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
		p.Logger.WarnContext(ctx, "judging attempt failed", "submission_id", id, "attempt", claimed.Attempt, "error", err)
		if int(claimed.Attempt) >= p.MaxAttempts {
			return p.fail(ctx, id, claimed.UserID)
		}
		if _, err := q.RequeueSubmission(ctx, id); err != nil {
			return "", fmt.Errorf("requeue: %w", err)
		}
		return Requeued, nil
	}

	stored, err := p.store(ctx, id, result)
	if err != nil {
		return "", err
	}
	if stored {
		p.publish(ctx, events.SubmissionUpdated{SubmissionID: id, UserID: claimed.UserID, Status: "done", Verdict: string(result.Verdict)})
		if claimed.ContestID.Valid {
			p.contestJudged(ctx, int(claimed.ContestID.Int64), claimed.UserID)
		}
	}
	// One line per verdict: the dashboard counts verdicts by language from these logs.
	p.Logger.InfoContext(ctx, "submission judged", "submission_id", id, "verdict", string(result.Verdict),
		"language", claimed.Language, "problem", prob.Slug, "attempt", claimed.Attempt)
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

func (p *Processor) fail(ctx context.Context, id uuid.UUID, userID string) (Outcome, error) {
	rows, err := p.Store.Queries.FailSubmission(ctx, id)
	if err != nil {
		return "", fmt.Errorf("fail submission: %w", err)
	}
	if rows > 0 {
		p.publish(ctx, events.SubmissionUpdated{SubmissionID: id, UserID: userID, Status: "failed", Verdict: string(judge.InternalError)})
	}
	return Failed, nil
}

// contestJudged updates the leaderboard cache and tells the contest's viewers. Best
// effort like publish: the leaderboard is computed from Postgres when the cache misses.
func (p *Processor) contestJudged(ctx context.Context, contestID int, userID string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	c, err := p.Store.Ent.Contest.Get(ctx, contestID)
	if err != nil {
		p.Logger.WarnContext(ctx, "load contest", "contest_id", contestID, "error", err)
		return
	}
	now := time.Now()
	if p.Now != nil {
		now = p.Now()
	}
	times := contest.Times{ID: c.ID, StartsAt: c.StartsAt, FreezeAt: c.FreezeAt, EndsAt: c.EndsAt}
	if p.Leaderboard != nil {
		if err := p.Leaderboard.Record(ctx, times, userID, now); err != nil {
			p.Logger.WarnContext(ctx, "record leaderboard", "contest", c.Slug, "error", err)
		}
	}
	// While frozen the public board does not change: no update reveals late solves.
	if p.Events != nil && times.PhaseAt(now) != contest.Frozen {
		if err := p.Events.ContestUpdated(ctx, events.DetailTypeLeaderboard, events.ContestUpdated{Contest: c.Slug}); err != nil {
			p.Logger.WarnContext(ctx, "publish leaderboard event", "contest", c.Slug, "error", err)
		}
	}
}

// publish sends an event without failing the judge: the result is already stored and
// the SPA still polls slowly, so a lost event only delays the update.
func (p *Processor) publish(ctx context.Context, e events.SubmissionUpdated) {
	if p.Events == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := p.Events.SubmissionUpdated(ctx, e); err != nil {
		p.Logger.WarnContext(ctx, "publish event", "submission_id", e.SubmissionID, "status", e.Status, "error", err)
	}
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
	if prob.CheckerTolerance != nil {
		out.Tolerance = *prob.CheckerTolerance
	}
	if prob.CheckerSource != nil {
		out.CheckerSource = *prob.CheckerSource
	}
	if err := out.Validate(); err != nil {
		return nil, fmt.Errorf("problem %s: %w", prob.Slug, err)
	}
	for _, tc := range cases {
		out.Cases = append(out.Cases, problem.Case{Name: strconv.Itoa(tc.Ordinal), Input: tc.Input, Expected: tc.ExpectedOutput})
	}
	return out, nil
}

// store writes the per-case results and the verdict atomically.
// store saves the results; false means another worker finished first (nothing stored).
func (p *Processor) store(ctx context.Context, id uuid.UUID, r judge.Result) (bool, error) {
	params := db.InsertSubmissionResultsParams{SubmissionID: id}
	var runtime, memory int64
	for _, c := range r.Cases {
		ordinal, err := strconv.ParseInt(c.Name, 10, 64)
		if err != nil {
			return false, fmt.Errorf("case name %q is not an ordinal: %w", c.Name, err)
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
		return false, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	qtx := p.Store.Queries.WithTx(tx)
	if len(params.Ordinals) > 0 {
		if err := qtx.InsertSubmissionResults(ctx, params); err != nil {
			return false, fmt.Errorf("insert results: %w", err)
		}
	}
	rows, err := qtx.CompleteSubmission(ctx, complete)
	if err != nil {
		return false, fmt.Errorf("complete: %w", err)
	}
	if rows == 0 {
		// The lease expired and another worker finished first; keep its result.
		return false, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
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
