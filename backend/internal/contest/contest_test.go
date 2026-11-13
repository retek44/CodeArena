package contest_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/retek44/CodeArena/backend/internal/contest"
	"github.com/retek44/CodeArena/backend/internal/store/db"
)

var start = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

func at(minute int) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: start.Add(time.Duration(minute)*time.Minute + 30*time.Second), Valid: true}
}

func TestPhaseFollowsTheClock(t *testing.T) {
	t.Parallel()
	times := contest.Times{StartsAt: start, FreezeAt: start.Add(90 * time.Minute), EndsAt: start.Add(2 * time.Hour)}
	tests := map[time.Duration]contest.Phase{
		-time.Second:                 contest.Scheduled,
		0:                            contest.Running,
		90*time.Minute - time.Second: contest.Running,
		90 * time.Minute:             contest.Frozen,
		2*time.Hour - time.Second:    contest.Frozen,
		2 * time.Hour:                contest.Ended,
	}
	for offset, want := range tests {
		if got := times.PhaseAt(start.Add(offset)); got != want {
			t.Errorf("at %v: phase %s, want %s", offset, got, want)
		}
		accepts := want == contest.Running || want == contest.Frozen
		if got := times.AcceptsSubmissions(start.Add(offset)); got != accepts {
			t.Errorf("at %v: accepts = %v, want %v", offset, got, accepts)
		}
	}
}

func TestICPCScoring(t *testing.T) {
	t.Parallel()
	none := pgtype.Timestamptz{}
	rows := contest.Build([]db.ContestStandingsRow{
		// alice: A at 10 after 1 reject, B at 50 -> solved 2, penalty 10+20+50 = 80
		{UserID: "a", Username: "alice", Label: "A", SolvedAt: at(10), Rejected: 1},
		{UserID: "a", Username: "alice", Label: "B", SolvedAt: at(50)},
		// bob: A at 20, B unsolved with 3 rejects (no penalty) -> solved 1, penalty 20
		{UserID: "b", Username: "bob", Label: "A", SolvedAt: at(20)},
		{UserID: "b", Username: "bob", Label: "B", SolvedAt: none, Rejected: 3, Pending: 1},
		// carol: A at 40, B at 40 -> solved 2, penalty 80, last solve 40 (ahead of alice: 50)
		{UserID: "c", Username: "carol", Label: "A", SolvedAt: at(40)},
		{UserID: "c", Username: "carol", Label: "B", SolvedAt: at(40)},
		// dave and erin: nothing -> tied last
		{UserID: "d", Username: "dave", Label: "A", SolvedAt: none},
		{UserID: "d", Username: "dave", Label: "B", SolvedAt: none},
		{UserID: "e", Username: "erin", Label: "A", SolvedAt: none},
		{UserID: "e", Username: "erin", Label: "B", SolvedAt: none},
	}, start)

	want := []struct {
		name            string
		rank, solved, p int
	}{
		{"carol", 1, 2, 80},
		{"alice", 2, 2, 80},
		{"bob", 3, 1, 20},
		{"dave", 4, 0, 0},
		{"erin", 4, 0, 0},
	}
	if len(rows) != len(want) {
		t.Fatalf("%d rows, want %d", len(rows), len(want))
	}
	for i, w := range want {
		r := rows[i]
		if r.Username != w.name || r.Rank != w.rank || r.Solved != w.solved || r.Penalty != w.p {
			t.Errorf("row %d = %s rank %d solved %d penalty %d, want %+v", i, r.Username, r.Rank, r.Solved, r.Penalty, w)
		}
	}
	bob := rows[2]
	if bob.Problems[1].SolvedMinute != nil || bob.Problems[1].Rejected != 3 || bob.Problems[1].Pending != 1 {
		t.Errorf("bob B = %+v", bob.Problems[1])
	}
	if m := rows[1].Problems[0].SolvedMinute; m == nil || *m != 10 {
		t.Errorf("alice A minute = %v, want 10", m)
	}
	if rows[0].Score() <= rows[2].Score() || rows[2].Score() <= rows[3].Score() {
		t.Error("Score must order by solved, then penalty")
	}
}

func TestResultMail(t *testing.T) {
	minute := 12
	subject, body := contest.ResultMail("Weekly 1", contest.Row{
		Rank: 2, Username: "alice", Solved: 1, Penalty: 32,
		Problems: []contest.Cell{{Label: "A", SolvedMinute: &minute, Rejected: 1}, {Label: "B"}},
	}, 5)
	if subject != "Weekly 1: you finished #2" {
		t.Errorf("subject = %q", subject)
	}
	for _, want := range []string{"#2 of 5", "1 solved and 32 penalty minutes", "A  solved at minute 12 (1 rejected before)", "B  not solved"} {
		if !strings.Contains(body, want) {
			t.Errorf("body misses %q:\n%s", want, body)
		}
	}
}
