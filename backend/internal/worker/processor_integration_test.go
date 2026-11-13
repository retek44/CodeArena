//go:build integration

package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/retek44/CodeArena/backend/internal/outbox"
	"github.com/retek44/CodeArena/backend/internal/protocol"
	"github.com/retek44/CodeArena/backend/internal/queue"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submissionresult"
	"github.com/retek44/CodeArena/backend/internal/store/storetest"
	"github.com/retek44/CodeArena/backend/internal/worker"
)

func TestMain(m *testing.M) {
	os.Exit(storetest.Main(m))
}

// fakeRunner answers like the runner: it "runs" a + b by reading the expected
// behaviour from the source code, so tests choose the outcome per submission.
type fakeRunner struct {
	calls atomic.Int32
	fail  atomic.Int32 // number of calls that return an infrastructure error
}

func (f *fakeRunner) Invoke(_ context.Context, req protocol.Request) (protocol.Response, error) {
	f.calls.Add(1)
	if f.fail.Load() > 0 {
		f.fail.Add(-1)
		return protocol.Response{}, errors.New("runner unavailable")
	}
	resp := protocol.Response{ProtocolVersion: protocol.Version, SubmissionID: req.SubmissionID,
		Compile: protocol.Compile{Status: protocol.CompileOK}}
	if strings.Contains(req.Source, "syntax error") {
		resp.Compile = protocol.Compile{Status: protocol.CompileError, Output: "SyntaxError: invalid syntax"}
		return resp, nil
	}
	for _, tc := range req.TestCases {
		out := "3\n"
		if strings.Contains(req.Source, "wrong") && tc.ID == "2" {
			out = "4\n"
		}
		resp.Results = append(resp.Results, protocol.CaseResult{
			ID: tc.ID, Status: protocol.StatusOK, CPUTimeMS: 10 + len(tc.ID), MemoryKB: 9000, Stdout: out,
		})
	}
	return resp, nil
}

type env struct {
	t      *testing.T
	store  *store.Store
	runner *fakeRunner
	proc   *worker.Processor
	prob   *ent.Problem
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, _ := storetest.New(t)
	ctx := t.Context()

	st.Ent.User.Create().SetID("user-1").SetUsername("alice").ExecX(ctx)
	p := st.Ent.Problem.Create().
		SetSlug("a-plus-b").SetTitle("A + B").SetDifficulty(problem.DifficultyEasy).
		SetStatementMd("Print a + b.").SetTimeLimitMs(1000).SetMemoryLimitMB(64).SetPublished(true).
		SaveX(ctx)
	for ordinal := 1; ordinal <= 3; ordinal++ {
		st.Ent.TestCase.Create().SetProblem(p).SetProblemVersion(1).SetOrdinal(ordinal).
			SetInput("1 2\n").SetExpectedOutput("3\n").SetIsSample(ordinal == 1).ExecX(ctx)
	}

	fr := &fakeRunner{}
	return &env{
		t:      t,
		store:  st,
		runner: fr,
		prob:   p,
		proc: &worker.Processor{
			Store:       st,
			Invoker:     fr,
			Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
			Lease:       time.Minute,
			MaxAttempts: 3,
		},
	}
}

func (e *env) submit(source string) uuid.UUID {
	e.t.Helper()
	return e.store.Ent.Submission.Create().
		SetUserID("user-1").SetProblem(e.prob).SetProblemVersion(1).
		SetLanguage(submission.LanguagePython).SetSource(source).
		SetIdempotencyKey(uuid.NewString()).SetRequestHash("h").
		SaveX(e.t.Context()).ID
}

func (e *env) reload(id uuid.UUID) *ent.Submission {
	e.t.Helper()
	return e.store.Ent.Submission.Query().Where(submission.ID(id)).
		WithResults(func(q *ent.SubmissionResultQuery) { q.Order(ent.Asc(submissionresult.FieldOrdinal)) }).
		OnlyX(e.t.Context())
}

func message(t *testing.T, id uuid.UUID) queue.Delivery {
	t.Helper()
	body, err := json.Marshal(outbox.SubmissionCreated{Version: 1, SubmissionID: id})
	if err != nil {
		t.Fatal(err)
	}
	return queue.Delivery{Body: body, ReceiveCount: 1}
}

func TestAcceptedSubmissionIsStoredWithResults(t *testing.T) {
	e := newEnv(t)
	id := e.submit("print(sum(map(int, input().split())))")

	if got := e.proc.Handle(t.Context(), message(t, id)); got != queue.Ack {
		t.Fatalf("decision = %v, want Ack", got)
	}

	sub := e.reload(id)
	if sub.Status != submission.StatusDone || sub.Verdict == nil || *sub.Verdict != submission.VerdictAC {
		t.Fatalf("status=%s verdict=%v, want done AC", sub.Status, sub.Verdict)
	}
	if len(sub.Edges.Results) != 3 || sub.RuntimeMs == nil || *sub.RuntimeMs != 11 {
		t.Errorf("results=%d runtime=%v, want 3 results and max runtime 11", len(sub.Edges.Results), sub.RuntimeMs)
	}
}

func TestWrongAnswerAndCompileError(t *testing.T) {
	e := newEnv(t)
	wrong := e.submit("wrong answer on case 2")
	broken := e.submit("syntax error")

	e.proc.Handle(t.Context(), message(t, wrong))
	e.proc.Handle(t.Context(), message(t, broken))

	w := e.reload(wrong)
	if w.Verdict == nil || *w.Verdict != submission.VerdictWA || w.Edges.Results[1].Verdict != submissionresult.VerdictWA {
		t.Errorf("wrong: verdict=%v results=%+v, want WA on case 2", w.Verdict, w.Edges.Results)
	}
	b := e.reload(broken)
	if b.Verdict == nil || *b.Verdict != submission.VerdictCE || b.CompileOutput == nil || len(b.Edges.Results) != 0 {
		t.Errorf("broken: verdict=%v compile=%v results=%d, want CE with output and no results",
			b.Verdict, b.CompileOutput, len(b.Edges.Results))
	}
}

func TestDuplicateDeliveryIsJudgedOnce(t *testing.T) {
	e := newEnv(t)
	id := e.submit("print(3)")

	for range 3 {
		if got := e.proc.Handle(t.Context(), message(t, id)); got != queue.Ack {
			t.Fatalf("decision = %v, want Ack", got)
		}
	}
	if n := e.runner.calls.Load(); n != 1 {
		t.Errorf("runner called %d times, want 1", n)
	}
	if n := len(e.reload(id).Edges.Results); n != 3 {
		t.Errorf("%d results, want 3", n)
	}
}

func TestRunnerFailureIsRetriedThenSucceeds(t *testing.T) {
	e := newEnv(t)
	id := e.submit("print(3)")
	e.runner.fail.Store(1)

	if got := e.proc.Handle(t.Context(), message(t, id)); got != queue.Retry {
		t.Fatalf("first delivery: decision = %v, want Retry", got)
	}
	if sub := e.reload(id); sub.Status != submission.StatusQueued || sub.Attempt != 1 {
		t.Fatalf("after failure: status=%s attempt=%d, want queued and 1", sub.Status, sub.Attempt)
	}

	if got := e.proc.Handle(t.Context(), message(t, id)); got != queue.Ack {
		t.Fatalf("redelivery: decision = %v, want Ack", got)
	}
	if sub := e.reload(id); sub.Status != submission.StatusDone || sub.Attempt != 2 {
		t.Errorf("after retry: status=%s attempt=%d, want done and 2", sub.Status, sub.Attempt)
	}
}

func TestRetryBudgetEndsWithInternalError(t *testing.T) {
	e := newEnv(t)
	id := e.submit("print(3)")
	e.runner.fail.Store(100)

	decisions := []queue.Decision{}
	for range 3 {
		decisions = append(decisions, e.proc.Handle(t.Context(), message(t, id)))
	}
	if decisions[0] != queue.Retry || decisions[1] != queue.Retry || decisions[2] != queue.Ack {
		t.Fatalf("decisions = %v, want Retry, Retry, Ack", decisions)
	}
	sub := e.reload(id)
	if sub.Status != submission.StatusFailed || sub.Verdict == nil || *sub.Verdict != submission.VerdictIE {
		t.Errorf("status=%s verdict=%v, want failed IE", sub.Status, sub.Verdict)
	}
}

func TestCrashedWorkerLeaseIsTakenOver(t *testing.T) {
	e := newEnv(t)
	id := e.submit("print(3)")
	ctx := t.Context()

	// A worker claimed the submission and died: status running, lease still valid.
	if _, err := e.store.Pool.Exec(ctx, `UPDATE submissions SET status = 'running', attempt = 1 WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if got := e.proc.Handle(ctx, message(t, id)); got != queue.Retry {
		t.Fatalf("during lease: decision = %v, want Retry (do not drop the message)", got)
	}

	// The lease runs out; the redelivered message takes over.
	if _, err := e.store.Pool.Exec(ctx, `UPDATE submissions SET updated_at = now() - interval '2 minutes' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if got := e.proc.Handle(ctx, message(t, id)); got != queue.Ack {
		t.Fatalf("after lease: decision = %v, want Ack", got)
	}
	if sub := e.reload(id); sub.Status != submission.StatusDone || sub.Attempt != 2 {
		t.Errorf("status=%s attempt=%d, want done and 2", sub.Status, sub.Attempt)
	}
}

func TestInvalidMessagesAreDropped(t *testing.T) {
	e := newEnv(t)
	for name, body := range map[string]string{
		"not json":           "hello",
		"unknown version":    `{"v":2,"submission_id":"` + uuid.NewString() + `"}`,
		"missing id":         `{"v":1}`,
		"unknown submission": `{"v":1,"submission_id":"` + uuid.NewString() + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if got := e.proc.Handle(t.Context(), queue.Delivery{Body: []byte(body)}); got != queue.Ack {
				t.Errorf("decision = %v, want Ack (retrying cannot help)", got)
			}
		})
	}
	if n := e.runner.calls.Load(); n != 0 {
		t.Errorf("runner called %d times for invalid messages", n)
	}
}
