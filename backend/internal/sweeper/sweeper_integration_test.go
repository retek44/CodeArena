//go:build integration

package sweeper_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/retek44/CodeArena/backend/internal/outbox"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
	"github.com/retek44/CodeArena/backend/internal/store/storetest"
	"github.com/retek44/CodeArena/backend/internal/sweeper"
)

func TestMain(m *testing.M) {
	os.Exit(storetest.Main(m))
}

const (
	runningAfter = 10 * time.Minute
	queuedAfter  = 15 * time.Minute
	maxAttempts  = 3
)

type env struct {
	t       *testing.T
	st      *store.Store
	user    *ent.User
	problem *ent.Problem
	n       int
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, _ := storetest.New(t)
	ctx := t.Context()
	u := st.Ent.User.Create().SetID("cognito-sub-1").SetUsername("alice").SaveX(ctx)
	p := st.Ent.Problem.Create().
		SetSlug("a-plus-b").
		SetTitle("A + B").
		SetDifficulty(problem.DifficultyEasy).
		SetStatementMd("Print a + b.").
		SetTimeLimitMs(1000).
		SetMemoryLimitMB(64).
		SetPublished(true).
		SaveX(ctx)
	return &env{t: t, st: st, user: u, problem: p}
}

// submission creates a submission in the given state, last updated `age` ago.
func (e *env) submission(status submission.Status, attempt int, age time.Duration) uuid.UUID {
	e.t.Helper()
	e.n++
	sub := e.st.Ent.Submission.Create().
		SetUserID(e.user.ID).
		SetProblemID(e.problem.ID).
		SetProblemVersion(e.problem.Version).
		SetLanguage(submission.LanguagePython).
		SetSource("print(1)").
		SetIdempotencyKey(fmt.Sprintf("key-%d", e.n)).
		SetRequestHash(fmt.Sprintf("hash-%d", e.n)).
		SaveX(e.t.Context())
	_, err := e.st.Pool.Exec(e.t.Context(),
		`UPDATE submissions SET status = $2, attempt = $3, updated_at = now() - make_interval(secs => $4) WHERE id = $1`,
		sub.ID, string(status), attempt, age.Seconds())
	if err != nil {
		e.t.Fatal(err)
	}
	return sub.ID
}

func (e *env) sweeper() *sweeper.Sweeper {
	return &sweeper.Sweeper{
		Pool:         e.st.Pool,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		RunningAfter: runningAfter,
		QueuedAfter:  queuedAfter,
		MaxAttempts:  maxAttempts,
	}
}

// announced returns the submission IDs of all outbox messages, in insertion order.
func (e *env) announced() []uuid.UUID {
	e.t.Helper()
	var ids []uuid.UUID
	for _, m := range e.st.Ent.OutboxMessage.Query().AllX(e.t.Context()) {
		if m.Topic != outbox.TopicSubmissionCreated {
			e.t.Errorf("topic = %q, want %q", m.Topic, outbox.TopicSubmissionCreated)
		}
		// The worker must be able to read what the sweeper writes.
		var payload outbox.SubmissionCreated
		if err := json.Unmarshal(m.Payload, &payload); err != nil || payload.Version != 1 {
			e.t.Fatalf("payload %s: version %d, err %v", m.Payload, payload.Version, err)
		}
		ids = append(ids, payload.SubmissionID)
	}
	return ids
}

func (e *env) status(id uuid.UUID) *ent.Submission {
	return e.st.Ent.Submission.GetX(e.t.Context(), id)
}

func TestSweepRequeuesOnlyStuckSubmissions(t *testing.T) {
	e := newEnv(t)
	silentWorker := e.submission(submission.StatusRunning, 1, runningAfter+time.Minute)
	lostMessage := e.submission(submission.StatusQueued, 0, queuedAfter+time.Minute)
	validLease := e.submission(submission.StatusRunning, 1, time.Minute)
	waiting := e.submission(submission.StatusQueued, 0, time.Minute)
	done := e.submission(submission.StatusDone, 1, time.Hour)

	res, err := e.sweeper().SweepOnce(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if res != (sweeper.Result{Requeued: 2}) {
		t.Fatalf("result = %+v, want 2 requeued", res)
	}

	for _, id := range []uuid.UUID{silentWorker, lostMessage} {
		if got := e.status(id); got.Status != submission.StatusQueued || time.Since(got.UpdatedAt) > time.Minute {
			t.Errorf("%s: status %s, updated %s ago; want queued now", id, got.Status, time.Since(got.UpdatedAt))
		}
	}
	if got := e.status(silentWorker).Attempt; got != 1 {
		t.Errorf("attempt = %d, want 1 (the next claim counts the next attempt)", got)
	}
	for id, want := range map[uuid.UUID]submission.Status{validLease: submission.StatusRunning, waiting: submission.StatusQueued, done: submission.StatusDone} {
		if got := e.status(id).Status; got != want {
			t.Errorf("%s: status %s, want %s (untouched)", id, got, want)
		}
	}

	ids := e.announced()
	if len(ids) != 2 || !(ids[0] == silentWorker && ids[1] == lostMessage || ids[0] == lostMessage && ids[1] == silentWorker) {
		t.Errorf("announced %v, want exactly %s and %s", ids, silentWorker, lostMessage)
	}

	// The requeue refreshed updated_at: the next sweep waits for the full period again.
	if res, err := e.sweeper().SweepOnce(t.Context()); err != nil || res != (sweeper.Result{}) {
		t.Errorf("second sweep = %+v, %v; want nothing", res, err)
	}
}

func TestSweepFailsSubmissionsWithoutAttemptsLeft(t *testing.T) {
	e := newEnv(t)
	poison := e.submission(submission.StatusRunning, maxAttempts, runningAfter+time.Minute)
	retry := e.submission(submission.StatusRunning, maxAttempts-1, runningAfter+time.Minute)

	res, err := e.sweeper().SweepOnce(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if res != (sweeper.Result{Requeued: 1, Failed: 1}) {
		t.Fatalf("result = %+v, want 1 requeued and 1 failed", res)
	}

	got := e.status(poison)
	if got.Status != submission.StatusFailed || got.Verdict == nil || *got.Verdict != "IE" || got.JudgedAt == nil {
		t.Errorf("poison submission = status %s, verdict %v, judged_at %v; want failed, IE, set", got.Status, got.Verdict, got.JudgedAt)
	}
	if ids := e.announced(); len(ids) != 1 || ids[0] != retry {
		t.Errorf("announced %v, want only %s", ids, retry)
	}
}

func TestConcurrentSweepersAnnounceEachSubmissionOnce(t *testing.T) {
	e := newEnv(t)
	const stuck = 40
	for range stuck {
		e.submission(submission.StatusQueued, 0, queuedAfter+time.Minute)
	}

	// Four API tasks sweep at the same moment with small batches.
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		total int
	)
	for range 4 {
		wg.Go(func() {
			s := e.sweeper()
			s.BatchSize = 7
			for {
				res, err := s.SweepOnce(t.Context())
				if err != nil {
					t.Error(err)
					return
				}
				if res.Requeued == 0 {
					return
				}
				mu.Lock()
				total += res.Requeued
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	ids := e.announced()
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("%s announced twice", id)
		}
		seen[id] = true
	}
	if total != stuck || len(ids) != stuck {
		t.Errorf("requeued %d, announced %d; want %d each", total, len(ids), stuck)
	}
}
