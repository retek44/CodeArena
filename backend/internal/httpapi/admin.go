package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/retek44/CodeArena/backend/internal/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	entproblem "github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/testcase"
)

// Admin limits. Test cases travel to the runner in one Lambda request (6 MB), so their
// total size stays well below it; the request body may carry JSON escaping on top.
const (
	maxAdminRequestBytes = 12 << 20
	maxTestCaseBytes     = 4 << 20
	maxTestCases         = 100
	maxStatementChars    = 65536
	maxCheckerChars      = 65536
	maxTitleChars        = 200
	maxSlugChars         = 64
)

type testCaseJSON struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Sample bool   `json:"sample"`
}

// problemInputJSON is the body of POST /api/admin/problems and PUT /api/admin/problems/{slug}.
type problemInputJSON struct {
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
	TestCases        []testCaseJSON `json:"test_cases"`
}

type adminProblemJSON struct {
	problemInputJSON
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

type adminProblemSummaryJSON struct {
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	Difficulty string    `json:"difficulty"`
	Published  bool      `json:"published"`
	Version    int       `json:"version"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type meJSON struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Admin    bool   `json:"admin"`
}

// getMe: GET /api/users/me. The SPA shows the admin pages only to admins; the API
// checks the role on every admin request anyway.
func (s *server) getMe(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	s.writeJSON(w, http.StatusOK, meJSON{ID: p.UserID, Username: p.Username, Admin: p.Admin})
}

// requireAdmin lets only members of the admin group through (Cognito group "admins").
func (s *server) requireAdmin(next http.HandlerFunc) http.Handler {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if !principalFrom(r.Context()).Admin {
			s.writeProblem(w, http.StatusForbidden, "only admins may do this")
			return
		}
		next(w, r)
	})
}

// listAdminProblems: GET /api/admin/problems. Every problem, published or not.
func (s *server) listAdminProblems(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.Ent.Problem.Query().Order(ent.Asc(entproblem.FieldSlug)).All(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	items := make([]adminProblemSummaryJSON, 0, len(rows))
	for _, p := range rows {
		items = append(items, adminProblemSummaryJSON{
			Slug: p.Slug, Title: p.Title, Difficulty: string(p.Difficulty),
			Published: p.Published, Version: p.Version, UpdatedAt: p.UpdatedAt,
		})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// getAdminProblem: GET /api/admin/problems/{slug}. All fields and every test case of the
// current version, hidden ones included.
func (s *server) getAdminProblem(w http.ResponseWriter, r *http.Request) {
	p, err := s.Store.Ent.Problem.Query().Where(entproblem.Slug(r.PathValue("slug"))).Only(r.Context())
	if ent.IsNotFound(err) {
		s.writeProblem(w, http.StatusNotFound, "problem not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	body, err := s.adminProblemOf(r.Context(), s.Store.Ent, p)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, body)
}

// createProblem: POST /api/admin/problems. The problem starts at version 1.
func (s *server) createProblem(w http.ResponseWriter, r *http.Request) {
	in, err := decodeProblemInput(w, r)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	tx, err := s.Store.Ent.Tx(ctx)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	now := s.Now()
	create := tx.Problem.Create().
		SetSlug(in.Slug).
		SetVersion(1).
		SetCreatedAt(now).
		SetUpdatedAt(now)
	applyProblemFields(create.Mutation(), in)
	created, err := create.Save(ctx)
	if err == nil {
		err = insertTestCases(ctx, tx, created.ID, 1, in.TestCases)
	}
	if err == nil {
		err = tx.Commit()
	} else {
		_ = tx.Rollback()
	}
	if ent.IsConstraintError(err) {
		s.writeProblem(w, http.StatusConflict, "a problem with this slug exists")
		return
	}
	if ent.IsValidationError(err) {
		s.writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	s.Logger.InfoContext(ctx, "problem created", "admin", principalFrom(ctx).Username, "slug", created.Slug, "version", 1)
	w.Header().Set("Location", "/api/admin/problems/"+created.Slug)
	s.writeJSON(w, http.StatusCreated, adminProblemJSON{problemInputJSON: in, Version: 1, UpdatedAt: now})
}

// updateProblem: PUT /api/admin/problems/{slug}. Replaces the problem. A change that
// affects judging (limits, checker or test cases) creates a new version: new submissions
// use the new test cases, earlier ones keep theirs. Title, statement, difficulty and
// publishing do not change the version.
func (s *server) updateProblem(w http.ResponseWriter, r *http.Request) {
	in, err := decodeProblemInput(w, r)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Slug != r.PathValue("slug") {
		s.writeProblem(w, http.StatusBadRequest, "slug cannot be changed")
		return
	}
	ctx := r.Context()
	tx, err := s.Store.Ent.Tx(ctx)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	body, status, err := s.updateProblemTx(ctx, tx, in)
	if err == nil && status == 0 {
		err = tx.Commit()
	} else {
		_ = tx.Rollback()
	}
	switch {
	case ent.IsValidationError(err):
		s.writeProblem(w, http.StatusBadRequest, err.Error())
	case err != nil:
		s.internalError(w, r, err)
	case status == http.StatusNotFound:
		s.writeProblem(w, status, "problem not found")
	case status == http.StatusConflict:
		s.writeProblem(w, status, "the problem was changed at the same time; load it again")
	default:
		s.Logger.InfoContext(ctx, "problem updated", "admin", principalFrom(ctx).Username, "slug", in.Slug, "version", body.Version)
		s.writeJSON(w, http.StatusOK, body)
	}
}

func (s *server) updateProblemTx(ctx context.Context, tx *ent.Tx, in problemInputJSON) (adminProblemJSON, int, error) {
	current, err := tx.Problem.Query().Where(entproblem.Slug(in.Slug)).Only(ctx)
	if ent.IsNotFound(err) {
		return adminProblemJSON{}, http.StatusNotFound, nil
	}
	if err != nil {
		return adminProblemJSON{}, 0, err
	}
	before, err := s.adminProblemOf(ctx, tx.Client(), current)
	if err != nil {
		return adminProblemJSON{}, 0, err
	}

	version := current.Version
	if judgingChanged(before.problemInputJSON, in) {
		version++
		if err := insertTestCases(ctx, tx, current.ID, version, in.TestCases); err != nil {
			return adminProblemJSON{}, 0, err
		}
	}
	now := s.Now()
	// The version check makes concurrent saves safe: the second one updates no row.
	update := tx.Problem.Update().
		Where(entproblem.ID(current.ID), entproblem.Version(current.Version)).
		SetVersion(version).
		SetUpdatedAt(now)
	applyProblemFields(update.Mutation(), in)
	n, err := update.Save(ctx)
	if err != nil {
		return adminProblemJSON{}, 0, err
	}
	if n == 0 {
		return adminProblemJSON{}, http.StatusConflict, nil
	}
	return adminProblemJSON{problemInputJSON: in, Version: version, UpdatedAt: now}, 0, nil
}

// applyProblemFields sets the editable fields; the checker settings that do not belong
// to the chosen checker are cleared.
func applyProblemFields(m *ent.ProblemMutation, in problemInputJSON) {
	m.SetTitle(in.Title)
	m.SetDifficulty(entproblem.Difficulty(in.Difficulty))
	m.SetStatementMd(in.StatementMD)
	m.SetTimeLimitMs(in.TimeLimitMS)
	m.SetMemoryLimitMB(in.MemoryLimitMB)
	m.SetChecker(entproblem.Checker(in.Checker))
	m.SetPublished(in.Published)
	if in.CheckerTolerance != nil {
		m.SetCheckerTolerance(*in.CheckerTolerance)
	} else {
		m.ClearCheckerTolerance()
	}
	if in.CheckerSource != nil {
		m.SetCheckerSource(*in.CheckerSource)
	} else {
		m.ClearCheckerSource()
	}
}

func insertTestCases(ctx context.Context, tx *ent.Tx, problemID, version int, cases []testCaseJSON) error {
	builders := make([]*ent.TestCaseCreate, 0, len(cases))
	for i, c := range cases {
		builders = append(builders, tx.TestCase.Create().
			SetProblemID(problemID).
			SetProblemVersion(version).
			SetOrdinal(i+1).
			SetInput(c.Input).
			SetExpectedOutput(c.Output).
			SetIsSample(c.Sample))
	}
	return tx.TestCase.CreateBulk(builders...).Exec(ctx)
}

func (s *server) adminProblemOf(ctx context.Context, client *ent.Client, p *ent.Problem) (adminProblemJSON, error) {
	cases, err := client.TestCase.Query().
		Where(testcase.ProblemID(p.ID), testcase.ProblemVersion(p.Version)).
		Order(ent.Asc(testcase.FieldOrdinal)).
		All(ctx)
	if err != nil {
		return adminProblemJSON{}, fmt.Errorf("test cases of %s: %w", p.Slug, err)
	}
	body := adminProblemJSON{
		problemInputJSON: problemInputJSON{
			Slug: p.Slug, Title: p.Title, Difficulty: string(p.Difficulty), StatementMD: p.StatementMd,
			TimeLimitMS: p.TimeLimitMs, MemoryLimitMB: p.MemoryLimitMB, Checker: string(p.Checker),
			CheckerTolerance: p.CheckerTolerance, CheckerSource: p.CheckerSource, Published: p.Published,
			TestCases: make([]testCaseJSON, 0, len(cases)),
		},
		Version:   p.Version,
		UpdatedAt: p.UpdatedAt,
	}
	for _, c := range cases {
		body.TestCases = append(body.TestCases, testCaseJSON{Input: c.Input, Output: c.ExpectedOutput, Sample: c.IsSample})
	}
	return body, nil
}

// judgingChanged reports whether a submission could get another verdict.
func judgingChanged(before, after problemInputJSON) bool {
	if before.TimeLimitMS != after.TimeLimitMS || before.MemoryLimitMB != after.MemoryLimitMB ||
		before.Checker != after.Checker || !equalPtr(before.CheckerTolerance, after.CheckerTolerance) ||
		!equalPtr(before.CheckerSource, after.CheckerSource) || len(before.TestCases) != len(after.TestCases) {
		return true
	}
	for i := range before.TestCases {
		if before.TestCases[i] != after.TestCases[i] {
			return true
		}
	}
	return false
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func decodeProblemInput(w http.ResponseWriter, r *http.Request) (problemInputJSON, error) {
	var in problemInputJSON
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAdminRequestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return in, fmt.Errorf("request body is larger than %d MB", maxAdminRequestBytes>>20)
		}
		return in, errors.New("request body must be a JSON problem object")
	}
	in.Title = strings.TrimSpace(in.Title)
	return in, validateProblemInput(&in)
}

func validateProblemInput(in *problemInputJSON) error {
	switch {
	case len(in.Slug) > maxSlugChars || !slugPattern.MatchString(in.Slug):
		return errors.New("slug must be lowercase letters and digits separated by single hyphens (at most 64)")
	case in.Title == "" || len(in.Title) > maxTitleChars:
		return fmt.Errorf("title must not be empty or longer than %d bytes", maxTitleChars)
	case utf8.RuneCountInString(in.StatementMD) > maxStatementChars:
		return fmt.Errorf("statement_md must have at most %d characters", maxStatementChars)
	case in.TimeLimitMS < 100 || in.TimeLimitMS > 10000:
		return errors.New("time_limit_ms must be between 100 and 10000")
	case in.MemoryLimitMB < 16 || in.MemoryLimitMB > 512:
		return errors.New("memory_limit_mb must be between 16 and 512")
	case len(in.TestCases) == 0 || len(in.TestCases) > maxTestCases:
		return fmt.Errorf("test_cases must have 1 to %d entries", maxTestCases)
	}
	if entproblem.DifficultyValidator(entproblem.Difficulty(in.Difficulty)) != nil {
		return errors.New("difficulty must be easy, medium or hard")
	}

	// The checker settings that belong to another checker are dropped; a tolerance of 0
	// means the default one.
	if in.Checker != problem.CheckerFloat || (in.CheckerTolerance != nil && *in.CheckerTolerance == 0) {
		in.CheckerTolerance = nil
	}
	if in.Checker != problem.CheckerCustom {
		in.CheckerSource = nil
	}
	p := problem.Problem{Slug: in.Slug, Difficulty: in.Difficulty, Checker: in.Checker}
	if in.CheckerTolerance != nil {
		p.Tolerance = *in.CheckerTolerance
	}
	if in.CheckerSource != nil {
		if utf8.RuneCountInString(*in.CheckerSource) > maxCheckerChars {
			return fmt.Errorf("checker_source must have at most %d characters", maxCheckerChars)
		}
		p.CheckerSource = *in.CheckerSource
	}
	if err := p.Validate(); err != nil {
		return err
	}

	total := 0
	for _, c := range in.TestCases {
		total += len(c.Input) + len(c.Output)
	}
	if total > maxTestCaseBytes {
		return fmt.Errorf("test cases must have at most %d MB together", maxTestCaseBytes>>20)
	}
	return nil
}
