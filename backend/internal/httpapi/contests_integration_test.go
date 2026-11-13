//go:build integration

package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/retek44/CodeArena/backend/internal/httpapi"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
)

type contestBody struct {
	Slug     string `json:"slug"`
	Phase    string `json:"phase"`
	Problems []struct {
		Label string `json:"label"`
		Slug  string `json:"slug"`
	} `json:"problems"`
	Participants int   `json:"participants"`
	Registered   *bool `json:"registered"`
}

type leaderboardBody struct {
	Phase  string   `json:"phase"`
	Frozen bool     `json:"frozen"`
	Labels []string `json:"labels"`
	Rows   []struct {
		Rank     int    `json:"rank"`
		Username string `json:"username"`
		Solved   int    `json:"solved"`
		Penalty  int    `json:"penalty"`
	} `json:"rows"`
}

// contestAPI is an API with a movable clock and one contest: start at `start`,
// freeze after 90 minutes, end after 2 hours, problems A (a-plus-b) and B (pair-sum).
type contestAPI struct {
	*api
	now     *time.Time
	start   time.Time
	contest *ent.Contest
	b       *ent.Problem
}

func newContestAPI(t *testing.T) *contestAPI {
	t.Helper()
	start := time.Now().Add(time.Hour).Truncate(time.Minute)
	now := start.Add(-10 * time.Minute)
	a := newAPIWith(t, func(d *httpapi.Deps) { d.Now = func() time.Time { return now } })
	pa := a.problem("a-plus-b", problem.DifficultyEasy, true)
	pb := a.problem("pair-sum", problem.DifficultyMedium, true)
	a.problem("not-in-contest", problem.DifficultyEasy, true)
	ctx := t.Context()
	c := a.store.Ent.Contest.Create().SetSlug("weekly-1").SetTitle("Weekly 1").
		SetStartsAt(start).SetFreezeAt(start.Add(90 * time.Minute)).SetEndsAt(start.Add(2 * time.Hour)).SaveX(ctx)
	a.store.Ent.ContestProblem.Create().SetContestID(c.ID).SetProblemID(pa.ID).SetLabel("A").ExecX(ctx)
	a.store.Ent.ContestProblem.Create().SetContestID(c.ID).SetProblemID(pb.ID).SetLabel("B").ExecX(ctx)
	return &contestAPI{api: a, now: &now, start: start, contest: c, b: pb}
}

func (c *contestAPI) at(minutes int) { *c.now = c.start.Add(time.Duration(minutes) * time.Minute) }

func (c *contestAPI) submitIn(user, problemSlug string) *httptest.ResponseRecorder {
	return c.do(request{method: http.MethodPost, path: "/api/submissions", user: user, key: "contest-" + uuid.NewString(),
		body: map[string]any{"problem": problemSlug, "language": "python", "source": "print(1)", "contest": "weekly-1"}})
}

// judge marks the user's latest submission as judged.
func (c *contestAPI) judge(user string, verdict submission.Verdict) {
	c.t.Helper()
	ctx := c.t.Context()
	s := c.store.Ent.Submission.Query().Where(submission.UserID("dev-" + user)).Order(ent.Desc(submission.FieldCreatedAt)).FirstX(ctx)
	s.Update().SetStatus(submission.StatusDone).SetVerdict(verdict).ExecX(ctx)
}

func TestContestDetailHidesProblemsUntilTheStart(t *testing.T) {
	c := newContestAPI(t)

	list := c.do(request{method: http.MethodGet, path: "/api/contests"})
	expectStatus(t, list, http.StatusOK)
	if items := decode[page[contestBody]](t, list).Items; len(items) != 1 || items[0].Phase != "scheduled" {
		t.Errorf("list = %+v", items)
	}

	before := decode[contestBody](t, c.do(request{method: http.MethodGet, path: "/api/contests/weekly-1"}))
	if before.Phase != "scheduled" || len(before.Problems) != 0 || before.Registered != nil {
		t.Errorf("before the start = %+v, want no problems and no registered flag", before)
	}

	expectStatus(t, c.do(request{method: http.MethodPost, path: "/api/contests/weekly-1/register", user: "alice"}), http.StatusNoContent)
	expectStatus(t, c.do(request{method: http.MethodPost, path: "/api/contests/weekly-1/register", user: "alice"}), http.StatusNoContent)

	c.at(1)
	during := decode[contestBody](t, c.do(request{method: http.MethodGet, path: "/api/contests/weekly-1", user: "alice"}))
	if during.Phase != "running" || len(during.Problems) != 2 || during.Problems[0].Label != "A" ||
		during.Participants != 1 || during.Registered == nil || !*during.Registered {
		t.Errorf("during = %+v", during)
	}
	other := decode[contestBody](t, c.do(request{method: http.MethodGet, path: "/api/contests/weekly-1", user: "bob"}))
	if other.Registered == nil || *other.Registered {
		t.Errorf("bob registered = %v, want false", other.Registered)
	}
	expectStatus(t, c.do(request{method: http.MethodGet, path: "/api/contests/nope"}), http.StatusNotFound)
}

func TestContestSubmissionRules(t *testing.T) {
	c := newContestAPI(t)
	expectStatus(t, c.do(request{method: http.MethodPost, path: "/api/contests/weekly-1/register", user: "alice"}), http.StatusNoContent)

	expectStatus(t, c.submitIn("alice", "a-plus-b"), http.StatusConflict) // not started
	c.at(5)
	expectStatus(t, c.submitIn("bob", "a-plus-b"), http.StatusForbidden) // not registered
	expectStatus(t, c.submitIn("alice", "not-in-contest"), http.StatusNotFound)
	expectStatus(t, c.submitIn("alice", "a-plus-b"), http.StatusAccepted)

	stored := c.store.Ent.Submission.Query().Where(submission.UserID("dev-alice")).OnlyX(t.Context())
	if stored.ContestID == nil || *stored.ContestID != c.contest.ID {
		t.Errorf("contest_id = %v, want %d", stored.ContestID, c.contest.ID)
	}
	c.at(121)
	expectStatus(t, c.submitIn("alice", "a-plus-b"), http.StatusConflict) // ended
	expectStatus(t, c.do(request{method: http.MethodPost, path: "/api/contests/weekly-1/register", user: "bob"}), http.StatusConflict)
	expectStatus(t, c.do(request{method: http.MethodPost, path: "/api/contests/weekly-1/register"}), http.StatusUnauthorized)
}

func TestLeaderboardRanksAndFreezes(t *testing.T) {
	c := newContestAPI(t)
	for _, u := range []string{"alice", "bob", "carol"} {
		expectStatus(t, c.do(request{method: http.MethodPost, path: "/api/contests/weekly-1/register", user: u}), http.StatusNoContent)
	}
	c.at(10)
	c.submitIn("alice", "a-plus-b")
	c.judge("alice", submission.VerdictWA)
	c.at(15)
	c.submitIn("alice", "a-plus-b")
	c.judge("alice", submission.VerdictAC) // 15 + 20 = 35
	c.at(20)
	c.submitIn("bob", "a-plus-b")
	c.judge("bob", submission.VerdictAC) // 20
	c.at(95)                             // frozen
	c.submitIn("carol", "pair-sum")
	c.judge("carol", submission.VerdictAC)

	rec := c.do(request{method: http.MethodGet, path: "/api/contests/weekly-1/leaderboard"})
	expectStatus(t, rec, http.StatusOK)
	frozen := decode[leaderboardBody](t, rec)
	if !frozen.Frozen || frozen.Phase != "frozen" || len(frozen.Labels) != 2 || len(frozen.Rows) != 3 {
		t.Fatalf("frozen board = %+v", frozen)
	}
	if frozen.Rows[0].Username != "bob" || frozen.Rows[1].Username != "alice" || frozen.Rows[1].Penalty != 35 ||
		frozen.Rows[2].Username != "carol" || frozen.Rows[2].Solved != 0 {
		t.Errorf("frozen rows = %+v, want bob, alice (35), carol without her late solve", frozen.Rows)
	}
	if src := rec.Header().Get("X-Leaderboard-Source"); src != "database" {
		t.Errorf("source = %q, want database (no Valkey in this test)", src)
	}

	c.at(121)
	final := decode[leaderboardBody](t, c.do(request{method: http.MethodGet, path: "/api/contests/weekly-1/leaderboard"}))
	if final.Frozen || final.Rows[2].Username != "carol" || final.Rows[2].Solved != 1 || final.Rows[2].Rank != 3 {
		t.Errorf("final rows = %+v, want carol's solve counted after the end", final.Rows)
	}
}
