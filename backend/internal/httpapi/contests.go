package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/retek44/CodeArena/backend/internal/contest"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	entcontest "github.com/retek44/CodeArena/backend/internal/store/ent/contest"
	"github.com/retek44/CodeArena/backend/internal/store/ent/contestparticipant"
	"github.com/retek44/CodeArena/backend/internal/store/ent/contestproblem"
)

type contestSummaryJSON struct {
	Slug     string    `json:"slug"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"starts_at"`
	FreezeAt time.Time `json:"freeze_at"`
	EndsAt   time.Time `json:"ends_at"`
	Phase    string    `json:"phase"`
}

type contestProblemJSON struct {
	Label string `json:"label"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type contestJSON struct {
	contestSummaryJSON
	Description  string               `json:"description_md"`
	Problems     []contestProblemJSON `json:"problems"`
	Participants int                  `json:"participants"`
	// Registered is set only for signed-in callers.
	Registered *bool `json:"registered,omitempty"`
}

type leaderboardCellJSON struct {
	Label        string `json:"label"`
	SolvedMinute *int   `json:"solved_minute"`
	Rejected     int    `json:"rejected"`
	Pending      int    `json:"pending"`
}

type leaderboardRowJSON struct {
	Rank     int                   `json:"rank"`
	Username string                `json:"username"`
	Solved   int                   `json:"solved"`
	Penalty  int                   `json:"penalty"`
	Problems []leaderboardCellJSON `json:"problems"`
}

type leaderboardJSON struct {
	Contest string               `json:"contest"`
	Phase   string               `json:"phase"`
	Frozen  bool                 `json:"frozen"`
	Labels  []string             `json:"labels"`
	Rows    []leaderboardRowJSON `json:"rows"`
}

func timesOf(c *ent.Contest) contest.Times {
	return contest.Times{ID: c.ID, StartsAt: c.StartsAt, FreezeAt: c.FreezeAt, EndsAt: c.EndsAt}
}

func (s *server) toContestSummary(c *ent.Contest) contestSummaryJSON {
	return contestSummaryJSON{
		Slug:     c.Slug,
		Title:    c.Title,
		StartsAt: c.StartsAt.UTC(),
		FreezeAt: c.FreezeAt.UTC(),
		EndsAt:   c.EndsAt.UTC(),
		Phase:    string(timesOf(c).PhaseAt(s.Now())),
	}
}

// listContests: GET /api/contests, newest first. A handful of contests: no pagination.
func (s *server) listContests(w http.ResponseWriter, r *http.Request) {
	contests, err := s.Store.Ent.Contest.Query().Order(ent.Desc(entcontest.FieldStartsAt)).Limit(100).All(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	items := make([]contestSummaryJSON, 0, len(contests))
	for _, c := range contests {
		items = append(items, s.toContestSummary(c))
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) contestBySlug(ctx context.Context, slug string) (*ent.Contest, error) {
	if !slugPattern.MatchString(slug) || len(slug) > 64 {
		return nil, &ent.NotFoundError{}
	}
	return s.Store.Ent.Contest.Query().Where(entcontest.Slug(slug)).Only(ctx)
}

// getContest: GET /api/contests/{slug}. Problems stay hidden until the start.
func (s *server) getContest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, err := s.contestBySlug(ctx, r.PathValue("slug"))
	if ent.IsNotFound(err) {
		s.writeProblem(w, http.StatusNotFound, "contest not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := contestJSON{contestSummaryJSON: s.toContestSummary(c), Description: c.DescriptionMd, Problems: []contestProblemJSON{}}
	if timesOf(c).PhaseAt(s.Now()) != contest.Scheduled {
		entries, err := c.QueryProblems().WithProblem().Order(ent.Asc(contestproblem.FieldLabel)).All(ctx)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		for _, e := range entries {
			out.Problems = append(out.Problems, contestProblemJSON{Label: e.Label, Slug: e.Edges.Problem.Slug, Title: e.Edges.Problem.Title})
		}
	}
	if out.Participants, err = c.QueryParticipants().Count(ctx); err != nil {
		s.internalError(w, r, err)
		return
	}
	// The page is public; a signed-in caller also learns whether they are registered.
	if p, err := s.Auth.Authenticate(r); err == nil {
		registered, err := c.QueryParticipants().Where(contestparticipant.UserID(p.UserID)).Exist(ctx)
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		out.Registered = &registered
	}
	s.writeJSON(w, http.StatusOK, out)
}

// registerForContest: POST /api/contests/{slug}/register. Idempotent; allowed until the end.
func (s *server) registerForContest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	principal := principalFrom(ctx)
	c, err := s.contestBySlug(ctx, r.PathValue("slug"))
	if ent.IsNotFound(err) {
		s.writeProblem(w, http.StatusNotFound, "contest not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	now := s.Now()
	if timesOf(c).PhaseAt(now) == contest.Ended {
		s.writeProblem(w, http.StatusConflict, "the contest has ended")
		return
	}
	err = s.ensureUser(ctx, principal)
	if err == nil {
		err = s.Store.Ent.ContestParticipant.Create().SetContestID(c.ID).SetUserID(principal.UserID).SetRegisteredAt(now).
			OnConflictColumns(contestparticipant.FieldContestID, contestparticipant.FieldUserID).Ignore().Exec(ctx)
	}
	if err != nil {
		s.internalError(w, r, fmt.Errorf("register: %w", err))
		return
	}
	// The new participant appears on a cached leaderboard right away (with 0 solved).
	if err := s.Leaderboard.Record(ctx, timesOf(c), principal.UserID, now); err != nil {
		s.Logger.Warn("record participant", "contest", c.Slug, "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// getLeaderboard: GET /api/contests/{slug}/leaderboard. Frozen in the last phase.
func (s *server) getLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, err := s.contestBySlug(ctx, r.PathValue("slug"))
	if ent.IsNotFound(err) {
		s.writeProblem(w, http.StatusNotFound, "contest not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	now := s.Now()
	board, err := s.Leaderboard.Board(ctx, timesOf(c), now)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	labels, err := c.QueryProblems().Order(ent.Asc(contestproblem.FieldLabel)).Select(contestproblem.FieldLabel).Strings(ctx)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := leaderboardJSON{
		Contest: c.Slug,
		Phase:   string(timesOf(c).PhaseAt(now)),
		Frozen:  board.Frozen,
		Labels:  labels,
		Rows:    make([]leaderboardRowJSON, 0, len(board.Rows)),
	}
	for _, row := range board.Rows {
		cells := make([]leaderboardCellJSON, 0, len(row.Problems))
		for _, p := range row.Problems {
			cells = append(cells, leaderboardCellJSON{Label: p.Label, SolvedMinute: p.SolvedMinute, Rejected: p.Rejected, Pending: p.Pending})
		}
		out.Rows = append(out.Rows, leaderboardRowJSON{Rank: row.Rank, Username: row.Username, Solved: row.Solved, Penalty: row.Penalty, Problems: cells})
	}
	w.Header().Set("X-Leaderboard-Source", board.Source)
	s.writeJSON(w, http.StatusOK, out)
}
