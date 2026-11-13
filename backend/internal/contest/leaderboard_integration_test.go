//go:build integration

package contest_test

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	tcvalkey "github.com/testcontainers/testcontainers-go/modules/valkey"
	"github.com/valkey-io/valkey-go"

	"github.com/retek44/CodeArena/backend/internal/contest"
	"github.com/retek44/CodeArena/backend/internal/events"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	entcontest "github.com/retek44/CodeArena/backend/internal/store/ent/contest"
	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
	"github.com/retek44/CodeArena/backend/internal/store/storetest"
)

var valkeyURL string

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := tcvalkey.Run(ctx, "valkey/valkey:8.1-alpine")
	if err != nil {
		log.Fatalf("start valkey: %v", err)
	}
	valkeyURL, err = container.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("valkey url: %v", err)
	}
	code := storetest.Main(m)
	_ = container.Terminate(ctx)
	os.Exit(code)
}

type fixture struct {
	t     *testing.T
	st    *store.Store
	times contest.Times
	probs map[string]int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	st, _ := storetest.New(t)
	ctx := t.Context()
	start := time.Now().Add(-time.Hour).Truncate(time.Minute)
	c := st.Ent.Contest.Create().SetSlug("weekly-" + uuid.NewString()[:8]).SetTitle("Weekly").
		SetStartsAt(start).SetFreezeAt(start.Add(90 * time.Minute)).SetEndsAt(start.Add(2 * time.Hour)).SaveX(ctx)
	f := &fixture{t: t, st: st, probs: map[string]int{},
		times: contest.Times{ID: c.ID, StartsAt: c.StartsAt, FreezeAt: c.FreezeAt, EndsAt: c.EndsAt}}
	for _, label := range []string{"A", "B"} {
		p := st.Ent.Problem.Create().SetSlug("p-" + strings.ToLower(label) + "-" + uuid.NewString()[:6]).SetTitle(label).
			SetDifficulty(problem.DifficultyEasy).SetStatementMd("s").SetTimeLimitMs(1000).SetMemoryLimitMB(64).
			SetPublished(true).SaveX(ctx)
		st.Ent.ContestProblem.Create().SetContestID(c.ID).SetProblemID(p.ID).SetLabel(label).ExecX(ctx)
		f.probs[label] = p.ID
	}
	for _, u := range []string{"alice", "bob"} {
		st.Ent.User.Create().SetID(u).SetUsername(u).ExecX(ctx)
		st.Ent.ContestParticipant.Create().SetContestID(c.ID).SetUserID(u).ExecX(ctx)
	}
	return f
}

// submit stores a judged contest submission made `minute` minutes after the start.
func (f *fixture) submit(user, label string, minute int, verdict submission.Verdict) {
	f.t.Helper()
	f.st.Ent.Submission.Create().SetUserID(user).SetProblemID(f.probs[label]).SetProblemVersion(1).
		SetContestID(f.times.ID).SetLanguage(submission.LanguagePython).SetSource("x").
		SetIdempotencyKey(uuid.NewString()).SetRequestHash("h").
		SetStatus(submission.StatusDone).SetVerdict(verdict).
		SetCreatedAt(f.times.StartsAt.Add(time.Duration(minute) * time.Minute)).
		ExecX(f.t.Context())
}

func (f *fixture) board(lb *contest.Leaderboard, now time.Time) contest.Board {
	f.t.Helper()
	b, err := lb.Board(f.t.Context(), f.times, now)
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}

func newCache(t *testing.T) valkey.Client {
	t.Helper()
	opt, err := valkey.ParseURL(valkeyURL)
	if err != nil {
		t.Fatal(err)
	}
	client, err := valkey.NewClient(opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestCacheMatchesDatabaseAndFollowsRecords(t *testing.T) {
	f := newFixture(t)
	lb := &contest.Leaderboard{Queries: f.st.Queries, Cache: newCache(t), Logger: logger()}
	now := f.times.StartsAt.Add(30 * time.Minute)
	f.submit("alice", "A", 5, submission.VerdictWA)
	f.submit("alice", "A", 12, submission.VerdictAC)
	f.submit("bob", "A", 8, submission.VerdictAC)

	first := f.board(lb, now)
	if first.Source != contest.SourceDatabase || first.Rows[0].Username != "bob" || first.Rows[1].Penalty != 32 {
		t.Fatalf("first read = %+v", first)
	}
	cached := f.board(lb, now)
	if cached.Source != contest.SourceCache || len(cached.Rows) != 2 || cached.Rows[0].Username != "bob" {
		t.Fatalf("second read = %+v, want the same board from the cache", cached)
	}

	// alice solves B: the worker records her row, the cache follows without a rebuild.
	f.submit("alice", "B", 20, submission.VerdictAC)
	if err := lb.Record(t.Context(), f.times, "alice", now); err != nil {
		t.Fatal(err)
	}
	after := f.board(lb, now)
	if after.Source != contest.SourceCache || after.Rows[0].Username != "alice" || after.Rows[0].Solved != 2 {
		t.Errorf("after record = %+v, want alice first with 2 solved from the cache", after.Rows)
	}
	db := f.board(&contest.Leaderboard{Queries: f.st.Queries, Logger: logger()}, now)
	if db.Rows[0].Penalty != after.Rows[0].Penalty || db.Rows[1].Penalty != after.Rows[1].Penalty {
		t.Errorf("cache %+v differs from database %+v", after.Rows, db.Rows)
	}
}

func TestFrozenBoardHidesLateSubmissions(t *testing.T) {
	f := newFixture(t)
	lb := &contest.Leaderboard{Queries: f.st.Queries, Cache: newCache(t), Logger: logger()}
	frozen := f.times.FreezeAt.Add(10 * time.Minute)
	f.submit("alice", "A", 10, submission.VerdictAC)
	f.submit("bob", "A", 95, submission.VerdictAC) // after the freeze

	if err := lb.Snapshot(t.Context(), f.times); err != nil {
		t.Fatal(err)
	}
	b := f.board(lb, frozen)
	if !b.Frozen || b.Source != contest.SourceCache {
		t.Fatalf("board = %+v, want the frozen snapshot", b)
	}
	for _, r := range b.Rows {
		if r.Username == "bob" && r.Solved != 0 {
			t.Errorf("bob's solve after the freeze is visible: %+v", r)
		}
	}
	// After the end the board is live again and shows everything.
	ended := f.board(lb, f.times.EndsAt.Add(time.Minute))
	if ended.Frozen {
		t.Error("the board must be unfrozen after the end")
	}
	for _, r := range ended.Rows {
		if r.Username == "bob" && r.Solved != 1 {
			t.Errorf("bob after the end = %+v, want 1 solved", r)
		}
	}
}

func TestValkeyOutageFallsBackToPostgres(t *testing.T) {
	f := newFixture(t)
	cache := newCache(t)
	cache.Close() // every command fails, like an unreachable Valkey
	lb := &contest.Leaderboard{Queries: f.st.Queries, Cache: cache, Logger: logger()}
	f.submit("alice", "A", 10, submission.VerdictAC)

	b := f.board(lb, f.times.StartsAt.Add(30*time.Minute))
	if b.Source != contest.SourceDatabase || b.Rows[0].Username != "alice" || b.Rows[0].Solved != 1 {
		t.Errorf("board = %+v, want alice first from the database", b)
	}
}

type fakeMailer struct{ sent map[string]string }

func (m *fakeMailer) Send(_ context.Context, to, subject, _ string) error {
	m.sent[to] = subject
	return nil
}

type fakeDirectory map[string]string

func (d fakeDirectory) Email(_ context.Context, userID string) (string, error) { return d[userID], nil }

type phaseEvents struct{ phases []string }

func (p *phaseEvents) SubmissionUpdated(context.Context, events.SubmissionUpdated) error { return nil }
func (p *phaseEvents) ContestUpdated(_ context.Context, detailType string, e events.ContestUpdated) error {
	p.phases = append(p.phases, detailType+"/"+e.Phase)
	return nil
}

func TestLifecycleSteps(t *testing.T) {
	f := newFixture(t)
	mailer := &fakeMailer{sent: map[string]string{}}
	ev := &phaseEvents{}
	lc := &contest.Lifecycle{
		Store:       f.st,
		Leaderboard: &contest.Leaderboard{Queries: f.st.Queries, Cache: newCache(t), Logger: logger()},
		Events:      ev,
		Mailer:      mailer,
		Directory:   fakeDirectory{"alice": "alice@example.com"}, // bob has no address
		Logger:      logger(),
		Now:         func() time.Time { return f.times.EndsAt.Add(3 * time.Minute) },
	}
	f.submit("alice", "A", 10, submission.VerdictAC)

	for _, step := range []string{contest.StepStart, contest.StepFreeze, contest.StepEnd, contest.StepFinalize} {
		if err := lc.Run(t.Context(), step, f.times.ID); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}
	if err := lc.Run(t.Context(), "explode", f.times.ID); err == nil {
		t.Error("an unknown step must fail")
	}

	c := f.st.Ent.Contest.GetX(t.Context(), f.times.ID)
	if c.Status != "final" {
		t.Errorf("status = %s, want final", c.Status)
	}
	if len(ev.phases) != 4 || ev.phases[3] != events.DetailTypeContestPhase+"/final" {
		t.Errorf("phase events = %v", ev.phases)
	}
	if len(mailer.sent) != 1 || mailer.sent["alice@example.com"] != "Weekly: you finished #1" {
		t.Errorf("mails = %v, want one to alice with rank 1", mailer.sent)
	}
}

type recordingScheduler struct{ slugs []string }

func (r *recordingScheduler) Schedule(_ context.Context, c *ent.Contest) error {
	r.slugs = append(r.slugs, c.Slug)
	return nil
}

func TestSeedContestsOnce(t *testing.T) {
	st, _ := storetest.New(t)
	ctx := t.Context()
	for _, slug := range []string{"a-plus-b", "pair-sum"} {
		st.Ent.Problem.Create().SetSlug(slug).SetTitle(slug).SetDifficulty(problem.DifficultyEasy).
			SetStatementMd("s").SetTimeLimitMs(1000).SetMemoryLimitMB(64).SetPublished(true).ExecX(ctx)
	}
	path := filepath.Join(t.TempDir(), "contests.json")
	if err := os.WriteFile(path, []byte(`[
		{"slug": "now", "title": "Now", "starts_in_minutes": 0, "freeze_after_minutes": 100, "duration_minutes": 120,
		 "problems": {"B": "pair-sum", "A": "a-plus-b"}},
		{"slug": "later", "title": "Later", "starts_in_minutes": 1440, "freeze_after_minutes": 60, "duration_minutes": 90,
		 "problems": {"A": "a-plus-b"}}
	]`), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 13, 10, 0, 30, 0, time.UTC)
	sched := &recordingScheduler{}

	for range 2 {
		if err := contest.SeedContests(ctx, st, path, now, sched, logger()); err != nil {
			t.Fatal(err)
		}
	}
	if len(sched.slugs) != 2 {
		t.Errorf("scheduled %v, want each contest once", sched.slugs)
	}
	later := st.Ent.Contest.Query().Where(entcontest.Slug("later")).WithProblems().OnlyX(ctx)
	if !later.StartsAt.Equal(now.Truncate(time.Minute).Add(24*time.Hour)) || later.EndsAt.Sub(later.FreezeAt) != 30*time.Minute {
		t.Errorf("later = %v .. %v .. %v", later.StartsAt, later.FreezeAt, later.EndsAt)
	}
	if n := st.Ent.ContestProblem.Query().CountX(ctx); n != 3 {
		t.Errorf("%d contest problems, want 3", n)
	}

	// An unknown problem fails the seed and leaves no half-created contest.
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte(`[{"slug": "bad", "title": "B", "freeze_after_minutes": 1, "duration_minutes": 2, "problems": {"A": "missing"}}]`), 0o600)
	if err := contest.SeedContests(ctx, st, bad, now, nil, logger()); err == nil {
		t.Error("expected an error for an unknown problem")
	}
	if st.Ent.Contest.Query().Where(entcontest.Slug("bad")).ExistX(ctx) {
		t.Error("the failed contest must be rolled back")
	}
	// No file: nothing to do.
	if err := contest.SeedContests(ctx, st, filepath.Join(t.TempDir(), "none.json"), now, nil, logger()); err != nil {
		t.Error(err)
	}
}
