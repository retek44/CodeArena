//go:build integration

package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/retek44/CodeArena/backend/internal/hint"
	"github.com/retek44/CodeArena/backend/internal/httpapi"
	"github.com/retek44/CodeArena/backend/internal/store/db"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
)

// recordingHinter remembers the requests it got and answers with a fixed hint.
type recordingHinter struct {
	mu   sync.Mutex
	reqs []hint.Request
	err  error
}

func (h *recordingHinter) Hint(_ context.Context, req hint.Request) (hint.Response, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reqs = append(h.reqs, req)
	if h.err != nil {
		return hint.Response{}, h.err
	}
	return hint.Response{Hint: "Check the largest input.", Status: hint.StatusOK, Model: "test-model"}, nil
}

func (h *recordingHinter) calls() []hint.Request {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hint.Request(nil), h.reqs...)
}

type hintBody struct {
	SubmissionID string `json:"submission_id"`
	Hint         string `json:"hint"`
	Status       string `json:"status"`
}

// judged stores a submission of user for p and marks it done with verdict; failing is
// the ordinal of the first failing test case (0: all cases passed).
func (a *api) judged(user, key string, verdict submission.Verdict, failing int) *ent.Submission {
	a.t.Helper()
	ctx := a.t.Context()
	rec := a.do(request{method: http.MethodPost, path: "/api/submissions", user: user, key: key, body: validSubmission()})
	expectStatus(a.t, rec, http.StatusAccepted)
	s := a.store.Ent.Submission.GetX(ctx, uuid.MustParse(decode[submissionBody](a.t, rec).ID))
	ordinals, verdicts := []int64{1}, []string{"AC"}
	if failing > 0 {
		ordinals, verdicts = []int64{1, int64(failing)}, []string{"AC", string(verdict)}
	}
	if failing == 1 {
		ordinals, verdicts = []int64{1}, []string{string(verdict)}
	}
	if err := a.store.Queries.InsertSubmissionResults(ctx, db.InsertSubmissionResultsParams{
		SubmissionID: s.ID,
		Ordinals:     ordinals,
		Verdicts:     verdicts,
		RuntimesMs:   make([]int64, len(ordinals)),
		MemoriesKb:   make([]int64, len(ordinals)),
	}); err != nil {
		a.t.Fatal(err)
	}
	return s.Update().SetStatus(submission.StatusDone).SetVerdict(verdict).SaveX(ctx)
}

func hintPath(s *ent.Submission) string {
	return "/api/submissions/" + s.ID.String() + "/hint"
}

func TestHintSendsOnlyVisibleTestData(t *testing.T) {
	hinter := &recordingHinter{}
	a := newAPIWith(t, func(d *httpapi.Deps) { d.Hinter = hinter })
	p := a.problem("a-plus-b", problem.DifficultyEasy, true)
	a.testCase(p, 1, 1, true)
	a.testCase(p, 1, 2, false)

	sample := a.judged("alice", "hint-sample", submission.VerdictWA, 1)
	hidden := a.judged("alice", "hint-hidden", submission.VerdictRE, 2)

	rec := a.do(request{method: http.MethodPost, path: hintPath(sample), user: "alice"})
	expectStatus(t, rec, http.StatusOK)
	if got := decode[hintBody](t, rec); got.Hint != "Check the largest input." || got.Status != "ok" || got.SubmissionID != sample.ID.String() {
		t.Errorf("hint = %+v", got)
	}
	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(hidden), user: "alice"}), http.StatusOK)

	calls := hinter.calls()
	if len(calls) != 2 {
		t.Fatalf("%d hinter calls, want 2", len(calls))
	}
	first, second := calls[0], calls[1]
	if first.Verdict != "WA" || first.Statement != "Statement of a-plus-b" || first.Source == "" {
		t.Errorf("request = %+v", first)
	}
	if fc := first.FailedCase; fc == nil || !fc.IsSample || fc.Input != "in-1-1" || fc.Expected != "out-1-1" {
		t.Errorf("sample failed case = %+v, want its input and expected output", fc)
	}
	if fc := second.FailedCase; fc == nil || fc.Ordinal != 2 || fc.IsSample || fc.Input != "" || fc.Expected != "" {
		t.Errorf("hidden failed case = %+v, want no test data", fc)
	}
}

func TestHintIsStoredOncePerSubmission(t *testing.T) {
	hinter := &recordingHinter{}
	a := newAPIWith(t, func(d *httpapi.Deps) { d.Hinter = hinter })
	a.problem("a-plus-b", problem.DifficultyEasy, true)
	s := a.judged("alice", "hint-once", submission.VerdictTLE, 1)

	// Parallel clicks: one stored hint, every caller gets it.
	var wg sync.WaitGroup
	codes := make([]int, 4)
	for i := range codes {
		wg.Go(func() {
			codes[i] = a.do(request{method: http.MethodPost, path: hintPath(s), user: "alice"}).Code
		})
	}
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("request %d: status %d", i, code)
		}
	}
	calls := len(hinter.calls())
	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(s), user: "alice"}), http.StatusOK)
	if len(hinter.calls()) != calls {
		t.Error("asking again must return the stored hint without a model call")
	}
	if n := a.store.Ent.Hint.Query().CountX(t.Context()); n != 1 {
		t.Errorf("%d hints stored, want 1", n)
	}
}

func TestHintRules(t *testing.T) {
	hinter := &recordingHinter{}
	a := newAPIWith(t, func(d *httpapi.Deps) { d.Hinter = hinter })
	a.problem("a-plus-b", problem.DifficultyEasy, true)
	accepted := a.judged("alice", "hint-key-ac", submission.VerdictAC, 0)
	compile := a.judged("alice", "hint-key-ce", submission.VerdictCE, 1)
	wrong := a.judged("alice", "hint-key-wa", submission.VerdictWA, 1)

	rec := a.do(request{method: http.MethodPost, path: "/api/submissions", user: "alice", key: "hint-queued", body: validSubmission()})
	queued := "/api/submissions/" + decode[submissionBody](t, rec).ID + "/hint"

	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(accepted), user: "alice"}), http.StatusConflict)
	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(compile), user: "alice"}), http.StatusConflict)
	expectStatus(t, a.do(request{method: http.MethodPost, path: queued, user: "alice"}), http.StatusConflict)
	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(wrong), user: "bob"}), http.StatusNotFound)
	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(wrong)}), http.StatusUnauthorized)
	expectStatus(t, a.do(request{method: http.MethodPost, path: "/api/submissions/nope/hint", user: "alice"}), http.StatusNotFound)
	if n := len(hinter.calls()); n != 0 {
		t.Errorf("%d hinter calls, want 0", n)
	}
}

func TestHintRateLimit(t *testing.T) {
	now := time.Now()
	a := newAPIWith(t, func(d *httpapi.Deps) {
		d.Hinter = &recordingHinter{}
		d.HintLimit = httpapi.RateLimit{Max: 2, Window: time.Hour}
		d.Now = func() time.Time { return now }
	})
	a.problem("a-plus-b", problem.DifficultyEasy, true)
	subs := make([]*ent.Submission, 3)
	for i := range subs {
		subs[i] = a.judged("alice", "hint-limit-"+strconv.Itoa(i), submission.VerdictWA, 1)
	}

	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(subs[0]), user: "alice"}), http.StatusOK)
	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(subs[1]), user: "alice"}), http.StatusOK)
	limited := a.do(request{method: http.MethodPost, path: hintPath(subs[2]), user: "alice"})
	expectStatus(t, limited, http.StatusTooManyRequests)
	if retry, err := strconv.Atoi(limited.Header().Get("Retry-After")); err != nil || retry < 1 || retry > 3600 {
		t.Errorf("Retry-After = %q, want 1..3600 seconds", limited.Header().Get("Retry-After"))
	}
	// A stored hint does not count again.
	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(subs[0]), user: "alice"}), http.StatusOK)

	now = now.Add(time.Hour + time.Second)
	expectStatus(t, a.do(request{method: http.MethodPost, path: hintPath(subs[2]), user: "alice"}), http.StatusOK)
}

func TestHintUnavailable(t *testing.T) {
	off := newAPI(t)
	off.problem("a-plus-b", problem.DifficultyEasy, true)
	s := off.judged("alice", "hint-off", submission.VerdictWA, 1)
	expectStatus(t, off.do(request{method: http.MethodPost, path: hintPath(s), user: "alice"}), http.StatusServiceUnavailable)

	failing := newAPIWith(t, func(d *httpapi.Deps) { d.Hinter = &recordingHinter{err: errors.New("throttled")} })
	failing.problem("a-plus-b", problem.DifficultyEasy, true)
	s = failing.judged("alice", "hint-fail", submission.VerdictWA, 1)
	expectStatus(t, failing.do(request{method: http.MethodPost, path: hintPath(s), user: "alice"}), http.StatusServiceUnavailable)
	if n := failing.store.Ent.Hint.Query().CountX(t.Context()); n != 0 {
		t.Errorf("%d hints stored after a failed model call, want 0", n)
	}
}
