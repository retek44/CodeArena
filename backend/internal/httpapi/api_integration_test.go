//go:build integration

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"

	"github.com/retek44/CodeArena/backend/internal/httpapi"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/db"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/tag"
	"github.com/retek44/CodeArena/backend/internal/store/storetest"
)

// contract validates every response of every integration test against
// contracts/openapi.yaml: the tests double as contract tests for the API.
var contract validator.Validator

func TestMain(m *testing.M) {
	spec, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "openapi.yaml"))
	if err != nil {
		log.Fatalf("read OpenAPI spec: %v", err)
	}
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		log.Fatalf("parse OpenAPI spec: %v", err)
	}
	v, errs := validator.NewValidator(doc)
	if len(errs) > 0 {
		log.Fatalf("build OpenAPI validator: %v", errors.Join(errs...))
	}
	contract = v
	os.Exit(storetest.Main(m))
}

type api struct {
	t       *testing.T
	handler http.Handler
	store   *store.Store
	queries *storetest.QueryCounter
}

func newAPI(t *testing.T) *api {
	t.Helper()
	return newAPIWith(t, nil)
}

// newAPIWith lets a test adjust the dependencies (clock, limits) before the handler is built.
func newAPIWith(t *testing.T, adjust func(*httpapi.Deps)) *api {
	t.Helper()
	st, counter := storetest.New(t)
	deps := httpapi.Deps{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Store:  st,
		Auth:   httpapi.DevAuthenticator{},
	}
	if adjust != nil {
		adjust(&deps)
	}
	return &api{t: t, handler: httpapi.NewHandler(deps), store: st, queries: counter}
}

type request struct {
	method string
	path   string
	user   string // X-Dev-User; empty means anonymous
	key    string // Idempotency-Key
	body   any
}

func (a *api) do(req request) *httptest.ResponseRecorder {
	a.t.Helper()
	var body io.Reader
	if req.body != nil {
		raw, err := json.Marshal(req.body)
		if err != nil {
			a.t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	r := httptest.NewRequestWithContext(a.t.Context(), req.method, req.path, body)
	if req.user != "" {
		r.Header.Set(httpapi.DevUserHeader, req.user)
	}
	if req.key != "" {
		r.Header.Set("Idempotency-Key", req.key)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, r)
	a.checkContract(req, rec)
	return rec
}

// checkContract fails the test if the response does not match the OpenAPI document
// (status code declared, content type, body schema with additionalProperties: false).
// Only responses are checked: some tests send invalid requests on purpose.
func (a *api) checkContract(req request, rec *httptest.ResponseRecorder) {
	a.t.Helper()
	vr := httptest.NewRequestWithContext(a.t.Context(), req.method, req.path, nil)
	resp := rec.Result()
	defer resp.Body.Close()
	ok, verrs := contract.ValidateHttpResponse(vr, resp)
	if ok {
		return
	}
	for _, e := range verrs {
		details := ""
		for _, se := range e.SchemaValidationErrors {
			details += "\n    " + se.Error()
		}
		a.t.Errorf("contract violation: %s %s -> %d: %s: %s%s",
			req.method, req.path, rec.Code, e.Message, e.Reason, details)
	}
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, want, rec.Body.String())
	}
	wantType := "application/json"
	if want >= 400 {
		wantType = "application/problem+json"
	}
	if got := rec.Header().Get("Content-Type"); got != wantType {
		t.Errorf("Content-Type = %q, want %q", got, wantType)
	}
}

// --- fixtures -----------------------------------------------------------------

func (a *api) problem(slug string, difficulty problem.Difficulty, published bool, tags ...string) *ent.Problem {
	a.t.Helper()
	ctx := a.t.Context()
	create := a.store.Ent.Problem.Create().
		SetSlug(slug).
		SetTitle(strings.ToUpper(slug)).
		SetDifficulty(difficulty).
		SetStatementMd("Statement of " + slug).
		SetTimeLimitMs(1000).
		SetMemoryLimitMB(64).
		SetPublished(published)
	for _, name := range tags {
		// Tags are shared between problems: create once, reuse afterwards.
		id := a.store.Ent.Tag.Create().SetName(name).OnConflictColumns(tag.FieldName).UpdateNewValues().IDX(ctx)
		create.AddTagIDs(id)
	}
	return create.SaveX(ctx)
}

func (a *api) testCase(p *ent.Problem, version, ordinal int, sample bool) {
	a.t.Helper()
	a.store.Ent.TestCase.Create().
		SetProblem(p).
		SetProblemVersion(version).
		SetOrdinal(ordinal).
		SetInput(fmt.Sprintf("in-%d-%d", version, ordinal)).
		SetExpectedOutput(fmt.Sprintf("out-%d-%d", version, ordinal)).
		SetIsSample(sample).
		ExecX(a.t.Context())
}

// --- JSON shapes (subset of contracts/openapi.yaml) ----------------------------

type problemSummary struct {
	Slug       string   `json:"slug"`
	Difficulty string   `json:"difficulty"`
	Tags       []string `json:"tags"`
}

type page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

type problemBody struct {
	Slug    string `json:"slug"`
	Samples []struct {
		Input  string `json:"input"`
		Output string `json:"output"`
	} `json:"samples"`
}

type submissionBody struct {
	ID       string  `json:"id"`
	Problem  string  `json:"problem"`
	Language string  `json:"language"`
	Status   string  `json:"status"`
	Verdict  *string `json:"verdict"`
	Cases    []struct {
		Index   int    `json:"index"`
		Verdict string `json:"verdict"`
	} `json:"cases"`
}

// --- problems -------------------------------------------------------------------

func TestListProblemsPaginatesPublishedOnly(t *testing.T) {
	a := newAPI(t)
	for i := 1; i <= 5; i++ {
		a.problem(fmt.Sprintf("p-%d", i), problem.DifficultyEasy, true, "math", fmt.Sprintf("tag-%d", i))
	}
	a.problem("draft", problem.DifficultyEasy, false)

	var slugs []string
	path := "/api/problems?limit=2"
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("pagination does not terminate")
		}
		a.queries.Reset()
		rec := a.do(request{method: http.MethodGet, path: path})
		expectStatus(t, rec, http.StatusOK)
		// N+1 guard: problems + tags, independent of the page size.
		if n := a.queries.Count(); n > 2 {
			t.Errorf("page %d used %d queries, want at most 2", pages, n)
		}

		body := decode[page[problemSummary]](t, rec)
		for _, p := range body.Items {
			slugs = append(slugs, p.Slug)
			if len(p.Tags) != 2 {
				t.Errorf("%s has tags %v, want 2", p.Slug, p.Tags)
			}
		}
		if body.NextCursor == nil {
			break
		}
		path = "/api/problems?limit=2&cursor=" + *body.NextCursor
	}

	want := []string{"p-1", "p-2", "p-3", "p-4", "p-5"}
	if strings.Join(slugs, ",") != strings.Join(want, ",") {
		t.Errorf("slugs = %v, want %v (no drafts, no duplicates)", slugs, want)
	}
}

func TestListProblemsFiltersAndValidates(t *testing.T) {
	a := newAPI(t)
	a.problem("easy-one", problem.DifficultyEasy, true)
	a.problem("hard-one", problem.DifficultyHard, true)

	rec := a.do(request{method: http.MethodGet, path: "/api/problems?difficulty=hard"})
	expectStatus(t, rec, http.StatusOK)
	if body := decode[page[problemSummary]](t, rec); len(body.Items) != 1 || body.Items[0].Slug != "hard-one" {
		t.Errorf("items = %+v, want only hard-one", body.Items)
	}

	for _, path := range []string{
		"/api/problems?difficulty=impossible",
		"/api/problems?limit=0",
		"/api/problems?limit=500",
		"/api/problems?cursor=garbage",
	} {
		t.Run(path, func(t *testing.T) {
			expectStatus(t, a.do(request{method: http.MethodGet, path: path}), http.StatusBadRequest)
		})
	}
}

func TestGetProblemReturnsOnlyCurrentSamples(t *testing.T) {
	a := newAPI(t)
	p := a.problem("a-plus-b", problem.DifficultyEasy, true)
	a.testCase(p, 1, 2, true)
	a.testCase(p, 1, 1, true)
	a.testCase(p, 1, 3, false) // hidden
	a.testCase(p, 2, 1, true)  // future version, not current

	rec := a.do(request{method: http.MethodGet, path: "/api/problems/a-plus-b"})
	expectStatus(t, rec, http.StatusOK)
	body := decode[problemBody](t, rec)
	if len(body.Samples) != 2 || body.Samples[0].Input != "in-1-1" || body.Samples[1].Output != "out-1-2" {
		t.Errorf("samples = %+v, want version 1 samples 1 and 2 in order", body.Samples)
	}
	if strings.Contains(rec.Body.String(), "in-1-3") {
		t.Error("hidden test case leaked")
	}

	a.problem("draft", problem.DifficultyEasy, false)
	expectStatus(t, a.do(request{method: http.MethodGet, path: "/api/problems/draft"}), http.StatusNotFound)
	expectStatus(t, a.do(request{method: http.MethodGet, path: "/api/problems/nope"}), http.StatusNotFound)
}

// --- submissions -----------------------------------------------------------------

func validSubmission() map[string]any {
	return map[string]any{"problem": "a-plus-b", "language": "python", "source": "print(1)"}
}

func TestCreateSubmissionValidation(t *testing.T) {
	a := newAPI(t)
	a.problem("a-plus-b", problem.DifficultyEasy, true)

	with := func(k string, v any) map[string]any {
		b := validSubmission()
		b[k] = v
		return b
	}
	tests := []struct {
		name string
		req  request
		want int
	}{
		{"anonymous", request{key: "key-00001", body: validSubmission()}, http.StatusUnauthorized},
		{"bad dev user", request{user: "Bad User", key: "key-00001", body: validSubmission()}, http.StatusUnauthorized},
		{"missing key", request{user: "alice", body: validSubmission()}, http.StatusBadRequest},
		{"short key", request{user: "alice", key: "k", body: validSubmission()}, http.StatusBadRequest},
		{"unknown field", request{user: "alice", key: "key-00001", body: with("extra", 1)}, http.StatusBadRequest},
		{"bad language", request{user: "alice", key: "key-00001", body: with("language", "cobol")}, http.StatusBadRequest},
		{"empty source", request{user: "alice", key: "key-00001", body: with("source", "")}, http.StatusBadRequest},
		{"source too long", request{user: "alice", key: "key-00001", body: with("source", strings.Repeat("x", 65537))}, http.StatusBadRequest},
		{"bad slug", request{user: "alice", key: "key-00001", body: with("problem", "../etc")}, http.StatusBadRequest},
		{"unknown problem", request{user: "alice", key: "key-00001", body: with("problem", "nope")}, http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.method = http.MethodPost
			tc.req.path = "/api/submissions"
			rec := a.do(tc.req)
			expectStatus(t, rec, tc.want)
			if tc.want == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("401 without WWW-Authenticate header")
			}
		})
	}

	if n := a.store.Ent.Submission.Query().CountX(t.Context()); n != 0 {
		t.Errorf("%d submissions stored by invalid requests, want 0", n)
	}
}

func TestCreateSubmissionIsIdempotent(t *testing.T) {
	a := newAPI(t)
	a.problem("a-plus-b", problem.DifficultyEasy, true)
	post := func(user, key string, body map[string]any) *httptest.ResponseRecorder {
		return a.do(request{method: http.MethodPost, path: "/api/submissions", user: user, key: key, body: body})
	}

	first := post("alice", "retry-key-1", validSubmission())
	expectStatus(t, first, http.StatusAccepted)
	created := decode[submissionBody](t, first)
	if created.Status != "queued" || created.Verdict != nil || created.Problem != "a-plus-b" || len(created.Cases) != 0 {
		t.Errorf("created = %+v, want queued, no verdict, no cases", created)
	}
	if loc := first.Header().Get("Location"); loc != "/api/submissions/"+created.ID {
		t.Errorf("Location = %q", loc)
	}

	// Network retry: same key, same body -> same submission.
	retry := post("alice", "retry-key-1", validSubmission())
	expectStatus(t, retry, http.StatusAccepted)
	if got := decode[submissionBody](t, retry); got.ID != created.ID {
		t.Errorf("retry created %s, want the original %s", got.ID, created.ID)
	}

	// Same key, different body -> conflict.
	changed := validSubmission()
	changed["source"] = "print(2)"
	expectStatus(t, post("alice", "retry-key-1", changed), http.StatusConflict)

	// Keys are per user.
	other := post("bob", "retry-key-1", validSubmission())
	expectStatus(t, other, http.StatusAccepted)
	if got := decode[submissionBody](t, other); got.ID == created.ID {
		t.Error("bob got alice's submission")
	}

	if n := a.store.Ent.Submission.Query().CountX(t.Context()); n != 2 {
		t.Errorf("%d submissions stored, want 2", n)
	}
	// Transactional outbox: exactly one message per stored submission, none for replays or conflicts.
	if n := a.store.Ent.OutboxMessage.Query().CountX(t.Context()); n != 2 {
		t.Errorf("%d outbox messages, want 2", n)
	}
}

func TestGetSubmissionIsPrivate(t *testing.T) {
	a := newAPI(t)
	a.problem("a-plus-b", problem.DifficultyEasy, true)
	rec := a.do(request{method: http.MethodPost, path: "/api/submissions", user: "alice", key: "private-key-1", body: validSubmission()})
	expectStatus(t, rec, http.StatusAccepted)
	created := decode[submissionBody](t, rec)

	sub := a.store.Ent.Submission.Query().OnlyX(t.Context())
	if err := a.store.Queries.InsertSubmissionResults(t.Context(), db.InsertSubmissionResultsParams{
		SubmissionID: sub.ID,
		Ordinals:     []int64{2, 1},
		Verdicts:     []string{"WA", "AC"},
		RuntimesMs:   []int64{5, 4},
		MemoriesKb:   []int64{100, 100},
	}); err != nil {
		t.Fatal(err)
	}

	path := "/api/submissions/" + created.ID
	own := a.do(request{method: http.MethodGet, path: path, user: "alice"})
	expectStatus(t, own, http.StatusOK)
	got := decode[submissionBody](t, own)
	if len(got.Cases) != 2 || got.Cases[0].Index != 1 || got.Cases[1].Verdict != "WA" {
		t.Errorf("cases = %+v, want ordered by index", got.Cases)
	}

	expectStatus(t, a.do(request{method: http.MethodGet, path: path, user: "bob"}), http.StatusNotFound)
	expectStatus(t, a.do(request{method: http.MethodGet, path: path}), http.StatusUnauthorized)
	expectStatus(t, a.do(request{method: http.MethodGet, path: "/api/submissions/not-a-uuid", user: "alice"}), http.StatusNotFound)
}

func TestListMySubmissionsNewestFirst(t *testing.T) {
	a := newAPI(t)
	a.problem("a-plus-b", problem.DifficultyEasy, true)
	a.problem("other", problem.DifficultyEasy, true)

	var created []string
	for i := range 5 {
		body := validSubmission()
		if i == 2 {
			body["problem"] = "other"
		}
		rec := a.do(request{method: http.MethodPost, path: "/api/submissions", user: "alice", key: fmt.Sprintf("list-key-%d", i), body: body})
		expectStatus(t, rec, http.StatusAccepted)
		created = append(created, decode[submissionBody](t, rec).ID)
	}
	expectStatus(t, a.do(request{method: http.MethodPost, path: "/api/submissions", user: "bob", key: "list-key-bob", body: validSubmission()}), http.StatusAccepted)

	var seen []string
	path := "/api/users/me/submissions?limit=2"
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("pagination does not terminate")
		}
		a.queries.Reset()
		rec := a.do(request{method: http.MethodGet, path: path, user: "alice"})
		expectStatus(t, rec, http.StatusOK)
		// N+1 guard: submissions + problems + results, independent of the page size.
		if n := a.queries.Count(); n > 3 {
			t.Errorf("page %d used %d queries, want at most 3", pages, n)
		}
		body := decode[page[submissionBody]](t, rec)
		for _, s := range body.Items {
			seen = append(seen, s.ID)
		}
		if body.NextCursor == nil {
			break
		}
		path = "/api/users/me/submissions?limit=2&cursor=" + *body.NextCursor
	}

	if len(seen) != 5 {
		t.Fatalf("got %d submissions, want alice's 5", len(seen))
	}
	for i, id := range seen {
		if want := created[len(created)-1-i]; id != want {
			t.Errorf("position %d = %s, want %s (newest first)", i, id, want)
		}
	}

	rec := a.do(request{method: http.MethodGet, path: "/api/users/me/submissions?problem=other", user: "alice"})
	expectStatus(t, rec, http.StatusOK)
	if body := decode[page[submissionBody]](t, rec); len(body.Items) != 1 || body.Items[0].ID != created[2] {
		t.Errorf("filtered items = %+v, want only the 'other' submission", body.Items)
	}
}

func TestSubmissionRateLimit(t *testing.T) {
	now := time.Now()
	a := newAPIWith(t, func(d *httpapi.Deps) {
		d.SubmissionLimit = httpapi.RateLimit{Max: 2, Window: time.Minute}
		d.Now = func() time.Time { return now }
	})
	a.problem("a-plus-b", problem.DifficultyEasy, true)
	post := func(user, key string) *httptest.ResponseRecorder {
		return a.do(request{method: http.MethodPost, path: "/api/submissions", user: user, key: key, body: validSubmission()})
	}

	expectStatus(t, post("alice", "limit-key-1"), http.StatusAccepted)
	expectStatus(t, post("alice", "limit-key-2"), http.StatusAccepted)

	limited := post("alice", "limit-key-3")
	expectStatus(t, limited, http.StatusTooManyRequests)
	retry, err := strconv.Atoi(limited.Header().Get("Retry-After"))
	if err != nil || retry < 1 || retry > 60 {
		t.Errorf("Retry-After = %q, want 1..60 seconds", limited.Header().Get("Retry-After"))
	}

	// Retries of accepted requests are not new submissions and stay allowed.
	expectStatus(t, post("alice", "limit-key-1"), http.StatusAccepted)
	// The limit is per user.
	expectStatus(t, post("bob", "limit-key-1"), http.StatusAccepted)

	// After the window has passed, alice can submit again.
	now = now.Add(61 * time.Second)
	expectStatus(t, post("alice", "limit-key-3"), http.StatusAccepted)

	if n := a.store.Ent.Submission.Query().CountX(t.Context()); n != 4 {
		t.Errorf("%d submissions stored, want 4 (alice 3, bob 1)", n)
	}
}

// TestContractValidatorCatchesViolations makes sure the contract check is not silently
// passing everything: hand-made responses that break the spec must be rejected.
func TestContractValidatorCatchesViolations(t *testing.T) {
	get := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/problems", nil)

	tests := map[string]struct {
		status      int
		contentType string
		body        string
	}{
		"undeclared field": {http.StatusOK, "application/json",
			`{"items":[],"next_cursor":null,"total":3}`},
		"wrong type": {http.StatusOK, "application/json",
			`{"items":[{"slug":"a-plus-b","title":"A","difficulty":"impossible"}],"next_cursor":null}`},
		"undeclared status": {http.StatusTeapot, "application/json", `{}`},
		"error not problem+json": {http.StatusBadRequest, "application/json",
			`{"type":"about:blank","title":"Bad Request","status":400}`},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rec.Header().Set("Content-Type", tc.contentType)
			rec.WriteHeader(tc.status)
			_, _ = rec.WriteString(tc.body)
			resp := rec.Result()
			defer resp.Body.Close()

			if ok, _ := contract.ValidateHttpResponse(get, resp); ok {
				t.Error("contract check accepted a response that breaks the spec")
			}
		})
	}
}
