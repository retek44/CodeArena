// Package contest holds the contest rules (ICPC scoring, phases) and the leaderboard
// (ADR-0016): computed in Postgres, cached in Valkey sorted sets.
package contest

import (
	"cmp"
	"slices"
	"time"

	"github.com/retek44/CodeArena/backend/internal/store/db"
)

// Phase of a contest. It follows the clock only, so every API task and the worker
// agree on it without coordination; the lifecycle steps (Step Functions) add side
// effects (events, the frozen snapshot, result e-mails), not correctness.
type Phase string

// Phases.
const (
	Scheduled Phase = "scheduled"
	Running   Phase = "running"
	// Frozen: the last part of the contest. Others see the leaderboard as it was at
	// freeze_at; submissions are still judged.
	Frozen Phase = "frozen"
	Ended  Phase = "ended"
)

// Times of one contest.
type Times struct {
	ID       int
	StartsAt time.Time
	FreezeAt time.Time
	EndsAt   time.Time
}

// PhaseAt returns the phase at now.
func (t Times) PhaseAt(now time.Time) Phase {
	switch {
	case now.Before(t.StartsAt):
		return Scheduled
	case now.Before(t.FreezeAt):
		return Running
	case now.Before(t.EndsAt):
		return Frozen
	default:
		return Ended
	}
}

// AcceptsSubmissions reports whether contest submissions are allowed at now.
func (t Times) AcceptsSubmissions(now time.Time) bool {
	p := t.PhaseAt(now)
	return p == Running || p == Frozen
}

// PenaltyPerReject is the ICPC penalty in minutes for each rejected submission
// before the first accepted one (only for solved problems).
const PenaltyPerReject = 20

// Cell is one participant's result on one problem.
type Cell struct {
	Label string `json:"label"`
	// SolvedMinute: minutes from the start to the first AC; nil while unsolved.
	SolvedMinute *int `json:"solved_minute,omitempty"`
	Rejected     int  `json:"rejected"`
	Pending      int  `json:"pending"`
}

// Row is one participant on the leaderboard.
type Row struct {
	Rank     int    `json:"rank"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Solved   int    `json:"solved"`
	// Penalty in minutes: solve times plus PenaltyPerReject per rejected attempt.
	Penalty int `json:"penalty"`
	// LastSolve breaks ties: the earlier last solve ranks first.
	LastSolve int    `json:"last_solve"`
	Problems  []Cell `json:"problems"`
}

// Score orders rows in a Valkey sorted set (higher is better): more solved first,
// then less penalty. Ties are resolved by Rank.
func (r Row) Score() float64 {
	return float64(r.Solved)*1e7 - float64(r.Penalty)
}

// Build turns standings rows (ordered by user, then label) into leaderboard rows.
func Build(rows []db.ContestStandingsRow, startsAt time.Time) []Row {
	var out []Row
	for _, s := range rows {
		if len(out) == 0 || out[len(out)-1].UserID != s.UserID {
			out = append(out, Row{UserID: s.UserID, Username: s.Username})
		}
		row := &out[len(out)-1]
		cell := Cell{Label: s.Label, Rejected: int(s.Rejected), Pending: int(s.Pending)}
		if s.SolvedAt.Valid {
			minute := max(0, int(s.SolvedAt.Time.Sub(startsAt)/time.Minute))
			cell.SolvedMinute = &minute
			row.Solved++
			row.Penalty += minute + PenaltyPerReject*cell.Rejected
			row.LastSolve = max(row.LastSolve, minute)
		}
		row.Problems = append(row.Problems, cell)
	}
	Rank(out)
	return out
}

// Rank sorts rows by ICPC rules and numbers them; equal results share a rank.
func Rank(rows []Row) {
	slices.SortStableFunc(rows, func(a, b Row) int {
		return cmp.Or(
			cmp.Compare(b.Solved, a.Solved),
			cmp.Compare(a.Penalty, b.Penalty),
			cmp.Compare(a.LastSolve, b.LastSolve),
			cmp.Compare(a.Username, b.Username),
		)
	})
	for i := range rows {
		if i > 0 && rows[i].Solved == rows[i-1].Solved && rows[i].Penalty == rows[i-1].Penalty &&
			rows[i].LastSolve == rows[i-1].LastSolve {
			rows[i].Rank = rows[i-1].Rank
		} else {
			rows[i].Rank = i + 1
		}
	}
}
