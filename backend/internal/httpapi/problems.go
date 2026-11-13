package httpapi

import (
	"net/http"

	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/testcase"
)

type problemSummaryJSON struct {
	Slug       string   `json:"slug"`
	Title      string   `json:"title"`
	Difficulty string   `json:"difficulty"`
	Tags       []string `json:"tags"`
}

type sampleCaseJSON struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type problemJSON struct {
	problemSummaryJSON
	StatementMD   string           `json:"statement_md"`
	TimeLimitMS   int              `json:"time_limit_ms"`
	MemoryLimitMB int              `json:"memory_limit_mb"`
	Samples       []sampleCaseJSON `json:"samples"`
}

type problemPageJSON struct {
	Items      []problemSummaryJSON `json:"items"`
	NextCursor *string              `json:"next_cursor"`
}

// listProblems: GET /api/problems. Two queries per page at most (problems + tags),
// whatever the page size; the integration tests assert this (N+1 guard).
func (s *server) listProblems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := parseLimit(q)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}

	query := s.Store.Ent.Problem.Query().Where(problem.Published(true))
	if d := q.Get("difficulty"); d != "" {
		if problem.DifficultyValidator(problem.Difficulty(d)) != nil {
			s.writeProblem(w, http.StatusBadRequest, "difficulty must be easy, medium or hard")
			return
		}
		query = query.Where(problem.DifficultyEQ(problem.Difficulty(d)))
	}
	if c := q.Get("cursor"); c != "" {
		afterID, err := decodeProblemCursor(c)
		if err != nil {
			s.writeProblem(w, http.StatusBadRequest, err.Error())
			return
		}
		query = query.Where(problem.IDGT(afterID))
	}

	// One extra row tells whether another page exists.
	rows, err := query.Order(ent.Asc(problem.FieldID)).Limit(limit + 1).WithTags().All(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	page := problemPageJSON{Items: []problemSummaryJSON{}}
	if len(rows) > limit {
		rows = rows[:limit]
		next := encodeProblemCursor(rows[len(rows)-1].ID)
		page.NextCursor = &next
	}
	for _, p := range rows {
		page.Items = append(page.Items, summaryOf(p))
	}
	s.writeJSON(w, http.StatusOK, page)
}

// getProblem: GET /api/problems/{slug}. Only sample test cases of the current
// version are returned; hidden cases never leave the database.
func (s *server) getProblem(w http.ResponseWriter, r *http.Request) {
	p, err := s.Store.Ent.Problem.Query().
		Where(problem.Slug(r.PathValue("slug")), problem.Published(true)).
		WithTags().
		Only(r.Context())
	if ent.IsNotFound(err) {
		s.writeProblem(w, http.StatusNotFound, "problem not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	samples, err := p.QueryTestCases().
		Where(testcase.IsSample(true), testcase.ProblemVersion(p.Version)).
		Order(ent.Asc(testcase.FieldOrdinal)).
		All(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	body := problemJSON{
		problemSummaryJSON: summaryOf(p),
		StatementMD:        p.StatementMd,
		TimeLimitMS:        p.TimeLimitMs,
		MemoryLimitMB:      p.MemoryLimitMB,
		Samples:            make([]sampleCaseJSON, 0, len(samples)),
	}
	for _, tc := range samples {
		body.Samples = append(body.Samples, sampleCaseJSON{Input: tc.Input, Output: tc.ExpectedOutput})
	}
	s.writeJSON(w, http.StatusOK, body)
}

func summaryOf(p *ent.Problem) problemSummaryJSON {
	tags := make([]string, 0, len(p.Edges.Tags))
	for _, t := range p.Edges.Tags {
		tags = append(tags, t.Name)
	}
	return problemSummaryJSON{Slug: p.Slug, Title: p.Title, Difficulty: string(p.Difficulty), Tags: tags}
}
