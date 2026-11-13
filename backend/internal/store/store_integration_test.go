//go:build integration

// Integration tests for the hot-path queries against a real PostgreSQL
// (never SQLite: they use RETURNING, unnest and ON CONFLICT). Run with:
//
//	go test -tags integration ./internal/store/...
package store_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/db"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
	"github.com/retek44/CodeArena/backend/internal/store/storetest"
)

func TestMain(m *testing.M) {
	os.Exit(storetest.Main(m))
}

// lease is how long a running submission belongs to its worker.
const lease = 5 * time.Minute

func claim(id uuid.UUID) db.ClaimSubmissionParams {
	return db.ClaimSubmissionParams{ID: id, Lease: pgtype.Interval{Microseconds: lease.Microseconds(), Valid: true}}
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, _ := storetest.New(t)
	return st
}

type fixture struct {
	user    *ent.User
	problem *ent.Problem
}

func seed(t *testing.T, st *store.Store) fixture {
	t.Helper()
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
	return fixture{user: u, problem: p}
}

func newSubmission(t *testing.T, st *store.Store, f fixture, key string) *ent.Submission {
	t.Helper()
	return st.Ent.Submission.Create().
		SetUserID(f.user.ID).
		SetProblemID(f.problem.ID).
		SetProblemVersion(f.problem.Version).
		SetLanguage(submission.LanguagePython).
		SetSource("print(sum(map(int, input().split())))").
		SetIdempotencyKey(key).
		SetRequestHash("hash-" + key).
		SaveX(t.Context())
}

func TestClaimSubmissionWinsExactlyOnce(t *testing.T) {
	st := newStore(t)
	sub := newSubmission(t, st, seed(t, st), "key-claim")

	// Simulates SQS delivering the same message to several workers at once.
	const workers = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		claimed int
		noRows  int
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := st.Queries.ClaimSubmission(context.WithoutCancel(t.Context()), claim(sub.ID))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				claimed++
			case errors.Is(err, pgx.ErrNoRows):
				noRows++
			default:
				t.Errorf("claim: %v", err)
			}
		}()
	}
	wg.Wait()

	if claimed != 1 || noRows != workers-1 {
		t.Fatalf("claimed=%d noRows=%d, want 1 and %d", claimed, noRows, workers-1)
	}
	got := st.Ent.Submission.GetX(t.Context(), sub.ID)
	if got.Status != submission.StatusRunning || got.Attempt != 1 {
		t.Errorf("status=%s attempt=%d, want running and 1", got.Status, got.Attempt)
	}
}

func TestCompleteSubmissionOnlyFromRunning(t *testing.T) {
	st := newStore(t)
	sub := newSubmission(t, st, seed(t, st), "key-complete")
	ctx := t.Context()

	params := db.CompleteSubmissionParams{
		ID:        sub.ID,
		Verdict:   "AC",
		RuntimeMs: pgtype.Int8{Int64: 12, Valid: true},
		MemoryKb:  pgtype.Int8{Int64: 9000, Valid: true},
	}

	if n, err := st.Queries.CompleteSubmission(ctx, params); err != nil || n != 0 {
		t.Fatalf("complete while queued: rows=%d err=%v, want 0 rows", n, err)
	}
	if _, err := st.Queries.ClaimSubmission(ctx, claim(sub.ID)); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if n, err := st.Queries.CompleteSubmission(ctx, params); err != nil || n != 1 {
		t.Fatalf("complete while running: rows=%d err=%v, want 1 row", n, err)
	}
	if n, err := st.Queries.CompleteSubmission(ctx, params); err != nil || n != 0 {
		t.Fatalf("second complete: rows=%d err=%v, want 0 rows", n, err)
	}

	got := st.Ent.Submission.GetX(ctx, sub.ID)
	if got.Status != submission.StatusDone || got.Verdict == nil || *got.Verdict != submission.VerdictAC {
		t.Errorf("status=%s verdict=%v, want done AC", got.Status, got.Verdict)
	}
	if got.JudgedAt == nil || got.RuntimeMs == nil || *got.RuntimeMs != 12 {
		t.Errorf("judged_at=%v runtime_ms=%v, want set and 12", got.JudgedAt, got.RuntimeMs)
	}
}

func TestInsertSubmissionResultsIsIdempotent(t *testing.T) {
	st := newStore(t)
	sub := newSubmission(t, st, seed(t, st), "key-results")
	ctx := t.Context()

	params := db.InsertSubmissionResultsParams{
		SubmissionID: sub.ID,
		Ordinals:     []int64{1, 2, 3},
		Verdicts:     []string{"AC", "WA", "SKIPPED"},
		RuntimesMs:   []int64{10, 11, 0},
		MemoriesKb:   []int64{9000, 9100, 0},
	}
	// A retried worker writes the same results again.
	for range 2 {
		if err := st.Queries.InsertSubmissionResults(ctx, params); err != nil {
			t.Fatalf("insert results: %v", err)
		}
	}

	results := sub.QueryResults().AllX(ctx)
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	byOrdinal := map[int]string{}
	for _, r := range results {
		byOrdinal[r.Ordinal] = string(r.Verdict)
	}
	if byOrdinal[1] != "AC" || byOrdinal[2] != "WA" || byOrdinal[3] != "SKIPPED" {
		t.Errorf("results = %v", byOrdinal)
	}
}

func TestIdempotencyKeyIsUniquePerUser(t *testing.T) {
	st := newStore(t)
	f := seed(t, st)
	newSubmission(t, st, f, "same-key")

	_, err := st.Ent.Submission.Create().
		SetUserID(f.user.ID).
		SetProblemID(f.problem.ID).
		SetProblemVersion(1).
		SetLanguage(submission.LanguagePython).
		SetSource("print(1)").
		SetIdempotencyKey("same-key").
		SetRequestHash("other").
		Save(t.Context())
	if !ent.IsConstraintError(err) {
		t.Fatalf("err = %v, want a constraint error", err)
	}

	other := st.Ent.User.Create().SetID("cognito-sub-2").SetUsername("bob").SaveX(t.Context())
	newSubmission(t, st, fixture{user: other, problem: f.problem}, "same-key")
}

func TestRequeueAndStuckSubmissions(t *testing.T) {
	st := newStore(t)
	sub := newSubmission(t, st, seed(t, st), "key-stuck")
	ctx := t.Context()

	if _, err := st.Queries.ClaimSubmission(ctx, claim(sub.ID)); err != nil {
		t.Fatalf("claim: %v", err)
	}
	// Pretend the worker stopped reporting ten minutes ago.
	if _, err := st.Pool.Exec(ctx, `UPDATE submissions SET updated_at = now() - interval '10 minutes' WHERE id = $1`, sub.ID); err != nil {
		t.Fatal(err)
	}

	stuck, err := st.Queries.ListStuckSubmissions(ctx, db.ListStuckSubmissionsParams{
		OlderThan: pgtype.Interval{Microseconds: (5 * time.Minute).Microseconds(), Valid: true},
		MaxRows:   10,
	})
	if err != nil {
		t.Fatalf("list stuck: %v", err)
	}
	if len(stuck) != 1 || stuck[0].ID != sub.ID || stuck[0].Attempt != 1 {
		t.Fatalf("stuck = %+v, want the claimed submission with attempt 1", stuck)
	}

	if n, err := st.Queries.RequeueSubmission(ctx, sub.ID); err != nil || n != 1 {
		t.Fatalf("requeue: rows=%d err=%v", n, err)
	}
	if _, err := st.Queries.ClaimSubmission(ctx, claim(sub.ID)); err != nil {
		t.Fatalf("claim after requeue: %v", err)
	}
	if got := st.Ent.Submission.GetX(ctx, sub.ID); got.Attempt != 2 {
		t.Errorf("attempt = %d, want 2", got.Attempt)
	}

	if _, err := st.Queries.ClaimSubmission(ctx, claim(uuid.New())); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("claim unknown id: err = %v, want pgx.ErrNoRows", err)
	}
}

func TestClaimAfterLeaseExpiry(t *testing.T) {
	st := newStore(t)
	sub := newSubmission(t, st, seed(t, st), "key-lease")
	ctx := t.Context()

	if _, err := st.Queries.ClaimSubmission(ctx, claim(sub.ID)); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	// Another worker while the lease is valid: no row.
	if _, err := st.Queries.ClaimSubmission(ctx, claim(sub.ID)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("claim during lease: err = %v, want pgx.ErrNoRows", err)
	}

	// The first worker crashed: its lease runs out.
	if _, err := st.Pool.Exec(ctx, `UPDATE submissions SET updated_at = now() - interval '6 minutes' WHERE id = $1`, sub.ID); err != nil {
		t.Fatal(err)
	}
	row, err := st.Queries.ClaimSubmission(ctx, claim(sub.ID))
	if err != nil {
		t.Fatalf("claim after lease expiry: %v", err)
	}
	if row.Attempt != 2 {
		t.Errorf("attempt = %d, want 2", row.Attempt)
	}

	// A finished submission is never claimed again, however old.
	params := db.CompleteSubmissionParams{ID: sub.ID, Verdict: "AC"}
	if _, err := st.Queries.CompleteSubmission(ctx, params); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE submissions SET updated_at = now() - interval '1 day' WHERE id = $1`, sub.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Queries.ClaimSubmission(ctx, claim(sub.ID)); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("claim of a done submission: err = %v, want pgx.ErrNoRows", err)
	}
}
