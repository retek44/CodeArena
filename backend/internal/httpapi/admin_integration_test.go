//go:build integration

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
	"github.com/retek44/CodeArena/backend/internal/store/ent/testcase"
)

type testCaseBody struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Sample bool   `json:"sample"`
}

type adminProblemBody struct {
	Slug             string         `json:"slug"`
	Title            string         `json:"title"`
	Difficulty       string         `json:"difficulty"`
	StatementMD      string         `json:"statement_md"`
	TimeLimitMS      int            `json:"time_limit_ms"`
	MemoryLimitMB    int            `json:"memory_limit_mb"`
	Checker          string         `json:"checker"`
	CheckerTolerance *float64       `json:"checker_tolerance"`
	CheckerSource    *string        `json:"checker_source"`
	Published        bool           `json:"published"`
	TestCases        []testCaseBody `json:"test_cases"`
	Version          int            `json:"version,omitempty"`
}

func newProblemInput(slug string) adminProblemBody {
	return adminProblemBody{
		Slug:          slug,
		Title:         "Double It",
		Difficulty:    "easy",
		StatementMD:   "# Double It\n\nPrint `2n`.",
		TimeLimitMS:   1000,
		MemoryLimitMB: 64,
		Checker:       "exact",
		TestCases: []testCaseBody{
			{Input: "2\n", Output: "4\n", Sample: true},
			{Input: "21\n", Output: "42\n"},
		},
	}
}

func TestMeTellsWhetherTheCallerIsAnAdmin(t *testing.T) {
	a := newAPI(t)

	rec := a.do(request{method: http.MethodGet, path: "/api/users/me", user: "alice"})
	expectStatus(t, rec, http.StatusOK)
	if me := decode[struct {
		Username string
		Admin    bool
	}](t, rec); me.Username != "alice" || me.Admin {
		t.Errorf("alice: %+v", me)
	}

	rec = a.do(request{method: http.MethodGet, path: "/api/users/me", user: "admin"})
	if me := decode[struct{ Admin bool }](t, rec); !me.Admin {
		t.Error("admin is not an admin")
	}

	expectStatus(t, a.do(request{method: http.MethodGet, path: "/api/users/me"}), http.StatusUnauthorized)
}

func TestAdminRoutesNeedTheAdminGroup(t *testing.T) {
	a := newAPI(t)
	a.problem("a-plus-b", problem.DifficultyEasy, true)

	for _, req := range []request{
		{method: http.MethodGet, path: "/api/admin/problems"},
		{method: http.MethodGet, path: "/api/admin/problems/a-plus-b"},
		{method: http.MethodPost, path: "/api/admin/problems", body: newProblemInput("double-it")},
		{method: http.MethodPut, path: "/api/admin/problems/a-plus-b", body: newProblemInput("a-plus-b")},
	} {
		expectStatus(t, a.do(req), http.StatusUnauthorized)
		req.user = "alice"
		expectStatus(t, a.do(req), http.StatusForbidden)
	}
	if n := a.store.Ent.Problem.Query().CountX(t.Context()); n != 1 {
		t.Errorf("%d problems, want 1: a non-admin changed something", n)
	}
}

func TestAdminCreatesPublishesAndVersionsAProblem(t *testing.T) {
	a := newAPI(t)
	ctx := t.Context()
	admin := func(method, path string, body any) request {
		return request{method: method, path: path, user: "admin", body: body}
	}

	// Created unpublished: only admins see it.
	in := newProblemInput("double-it")
	rec := a.do(admin(http.MethodPost, "/api/admin/problems", in))
	expectStatus(t, rec, http.StatusCreated)
	if got := decode[adminProblemBody](t, rec); got.Version != 1 || len(got.TestCases) != 2 {
		t.Fatalf("created: %+v", got)
	}
	expectStatus(t, a.do(request{method: http.MethodGet, path: "/api/problems/double-it"}), http.StatusNotFound)
	expectStatus(t, a.do(admin(http.MethodPost, "/api/admin/problems", in)), http.StatusConflict)

	// Publishing and a new statement keep the version.
	in.Published = true
	in.StatementMD = "# Double It\n\nRead `n`, print `2n`."
	rec = a.do(admin(http.MethodPut, "/api/admin/problems/double-it", in))
	expectStatus(t, rec, http.StatusOK)
	if got := decode[adminProblemBody](t, rec); got.Version != 1 {
		t.Errorf("version after publishing = %d, want 1", got.Version)
	}
	rec = a.do(request{method: http.MethodGet, path: "/api/problems/double-it"})
	expectStatus(t, rec, http.StatusOK)
	if got := decode[problemBody](t, rec); len(got.Samples) != 1 || got.Samples[0].Output != "4\n" {
		t.Errorf("public samples = %+v, want the one sample case", got.Samples)
	}

	// A submission now is judged with version 1.
	expectStatus(t, a.do(request{method: http.MethodPost, path: "/api/submissions", user: "alice", key: "admin-v1-key",
		body: map[string]string{"problem": "double-it", "language": "python", "source": "print(2*int(input()))"}}), http.StatusAccepted)

	// New test cases: version 2; version 1 keeps its test cases for earlier submissions.
	in.TestCases = append(in.TestCases, testCaseBody{Input: "-3\n", Output: "-6\n"})
	rec = a.do(admin(http.MethodPut, "/api/admin/problems/double-it", in))
	expectStatus(t, rec, http.StatusOK)
	if got := decode[adminProblemBody](t, rec); got.Version != 2 {
		t.Errorf("version after new test cases = %d, want 2", got.Version)
	}
	p := a.store.Ent.Problem.Query().Where(problem.Slug("double-it")).OnlyX(ctx)
	if n := p.QueryTestCases().Where(testcase.ProblemVersion(1)).CountX(ctx); n != 2 {
		t.Errorf("version 1 has %d test cases, want 2", n)
	}
	rec = a.do(admin(http.MethodGet, "/api/admin/problems/double-it", nil))
	expectStatus(t, rec, http.StatusOK)
	if got := decode[adminProblemBody](t, rec); len(got.TestCases) != 3 || got.TestCases[2].Output != "-6\n" {
		t.Errorf("current test cases = %+v", got.TestCases)
	}

	// A new limit is a new version too.
	in.TimeLimitMS = 2000
	rec = a.do(admin(http.MethodPut, "/api/admin/problems/double-it", in))
	if got := decode[adminProblemBody](t, rec); got.Version != 3 {
		t.Errorf("version after a new time limit = %d, want 3", got.Version)
	}

	expectStatus(t, a.do(request{method: http.MethodPost, path: "/api/submissions", user: "alice", key: "admin-v3-key",
		body: map[string]string{"problem": "double-it", "language": "python", "source": "print(2*int(input()))"}}), http.StatusAccepted)
	versions := a.store.Ent.Submission.Query().Order(submission.ByCreatedAt()).Select(submission.FieldProblemVersion).IntsX(ctx)
	if len(versions) != 2 || versions[0] != 1 || versions[1] != 3 {
		t.Errorf("submission problem versions = %v, want [1 3]", versions)
	}

	rec = a.do(admin(http.MethodGet, "/api/admin/problems", nil))
	expectStatus(t, rec, http.StatusOK)
	list := decode[struct {
		Items []struct {
			Slug      string
			Published bool
			Version   int
		}
	}](t, rec)
	if len(list.Items) != 1 || list.Items[0].Slug != "double-it" || !list.Items[0].Published || list.Items[0].Version != 3 {
		t.Errorf("admin list = %+v", list.Items)
	}
}

func TestAdminProblemValidation(t *testing.T) {
	a := newAPI(t)
	a.problem("a-plus-b", problem.DifficultyEasy, true)

	cases := map[string]func(*adminProblemBody){
		"bad slug":              func(p *adminProblemBody) { p.Slug = "Double It" },
		"empty title":           func(p *adminProblemBody) { p.Title = "  " },
		"time limit":            func(p *adminProblemBody) { p.TimeLimitMS = 50 },
		"memory limit":          func(p *adminProblemBody) { p.MemoryLimitMB = 4096 },
		"difficulty":            func(p *adminProblemBody) { p.Difficulty = "insane" },
		"checker":               func(p *adminProblemBody) { p.Checker = "regex" },
		"custom without source": func(p *adminProblemBody) { p.Checker = "custom" },
		"no test cases":         func(p *adminProblemBody) { p.TestCases = nil },
		"too much data": func(p *adminProblemBody) {
			p.TestCases = []testCaseBody{{Input: strings.Repeat("1", 4<<20), Output: "1\n"}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := newProblemInput("double-it")
			mutate(&in)
			expectStatus(t, a.do(request{method: http.MethodPost, path: "/api/admin/problems", user: "admin", body: in}), http.StatusBadRequest)
		})
	}

	renamed := newProblemInput("other-slug")
	expectStatus(t, a.do(request{method: http.MethodPut, path: "/api/admin/problems/a-plus-b", user: "admin", body: renamed}), http.StatusBadRequest)
	expectStatus(t, a.do(request{method: http.MethodPut, path: "/api/admin/problems/missing", user: "admin", body: newProblemInput("missing")}), http.StatusNotFound)

	// A custom checker keeps its source; settings of other checkers are dropped.
	custom := newProblemInput("custom-checker")
	source := "print('AC')\n"
	tolerance := 0.5
	custom.Checker, custom.CheckerSource, custom.CheckerTolerance = "custom", &source, &tolerance
	rec := a.do(request{method: http.MethodPost, path: "/api/admin/problems", user: "admin", body: custom})
	expectStatus(t, rec, http.StatusCreated)
	if got := decode[adminProblemBody](t, rec); got.CheckerSource == nil || *got.CheckerSource != source || got.CheckerTolerance != nil {
		t.Errorf("custom checker settings = %v %v", got.CheckerSource, got.CheckerTolerance)
	}
}
