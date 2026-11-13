package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/retek44/CodeArena/api/internal/store/ent"
	"github.com/retek44/CodeArena/api/internal/store/ent/problem"
	"github.com/retek44/CodeArena/api/internal/store/ent/submission"
	"github.com/retek44/CodeArena/api/internal/store/ent/submissionresult"
	"github.com/retek44/CodeArena/api/internal/store/ent/user"
)

const (
	maxSourceChars  = 65536
	maxRequestBytes = 512 << 10 // room for 64 Ki characters of UTF-8 source plus JSON escaping
)

var (
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)
	slugPattern           = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

type createSubmissionJSON struct {
	Problem  string `json:"problem"`
	Language string `json:"language"`
	Source   string `json:"source"`
}

type caseResultJSON struct {
	Index     int    `json:"index"`
	Verdict   string `json:"verdict"`
	RuntimeMS int    `json:"runtime_ms"`
	MemoryKB  int    `json:"memory_kb"`
}

type submissionJSON struct {
	ID            uuid.UUID        `json:"id"`
	Problem       string           `json:"problem"`
	Language      string           `json:"language"`
	Status        string           `json:"status"`
	Verdict       *string          `json:"verdict"`
	RuntimeMS     *int             `json:"runtime_ms"`
	MemoryKB      *int             `json:"memory_kb"`
	CompileOutput *string          `json:"compile_output"`
	CreatedAt     time.Time        `json:"created_at"`
	JudgedAt      *time.Time       `json:"judged_at"`
	Cases         []caseResultJSON `json:"cases"`
}

type submissionPageJSON struct {
	Items      []submissionJSON `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

// createSubmission: POST /api/submissions. Idempotent per (user, Idempotency-Key):
// a retry with the same body returns the original submission, a different body is 409.
func (s *server) createSubmission(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r.Context())

	key := r.Header.Get("Idempotency-Key")
	if !idempotencyKeyPattern.MatchString(key) {
		s.writeProblem(w, http.StatusBadRequest, "Idempotency-Key header is required: 8-128 characters of A-Z, a-z, 0-9, _ or -")
		return
	}

	req, err := decodeCreateSubmission(w, r)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}
	hash := requestHash(req)
	ctx := r.Context()

	// A retry of an earlier request is answered from the database and never rate limited.
	replay, err := s.Store.Ent.Submission.Query().
		Where(submission.UserID(principal.UserID), submission.IdempotencyKey(key)).
		Exist(ctx)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if replay {
		s.replaySubmission(w, r, principal.UserID, key, hash)
		return
	}

	wait, err := s.submissionRetryAfter(ctx, principal.UserID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		s.writeProblem(w, http.StatusTooManyRequests,
			fmt.Sprintf("at most %d submissions per %s", s.SubmissionLimit.Max, s.SubmissionLimit.Window))
		return
	}

	if err := s.Store.Ent.User.Create().
		SetID(principal.UserID).
		SetUsername(principal.Username).
		OnConflictColumns(user.FieldID).
		Ignore().
		Exec(ctx); err != nil {
		s.internalError(w, r, fmt.Errorf("ensure user: %w", err))
		return
	}

	p, err := s.Store.Ent.Problem.Query().
		Where(problem.Slug(req.Problem), problem.Published(true)).
		Only(ctx)
	if ent.IsNotFound(err) {
		s.writeProblem(w, http.StatusNotFound, "problem not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	created, err := s.Store.Ent.Submission.Create().
		SetUserID(principal.UserID).
		SetProblemID(p.ID).
		SetProblemVersion(p.Version).
		SetLanguage(submission.Language(req.Language)).
		SetSource(req.Source).
		SetIdempotencyKey(key).
		SetRequestHash(hash).
		// Same clock as the rate limit window, so Retry-After never exceeds the window.
		SetCreatedAt(s.Now()).
		Save(ctx)
	if ent.IsConstraintError(err) {
		s.replaySubmission(w, r, principal.UserID, key, hash)
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	// Phase 3 publishes the submission to SQS (transactional outbox); until then it stays queued.
	created.Edges.Problem = p
	w.Header().Set("Location", "/api/submissions/"+created.ID.String())
	s.writeJSON(w, http.StatusAccepted, toSubmissionJSON(created))
}

// submissionRetryAfter applies a sliding-window limit per user, counted from the
// submissions table (shared by all API tasks, no extra infrastructure). Two requests
// racing at the limit can both pass; the limit protects capacity, not money.
func (s *server) submissionRetryAfter(ctx context.Context, userID string) (time.Duration, error) {
	limit := s.SubmissionLimit
	now := s.Now()
	recent, err := s.Store.Ent.Submission.Query().
		Where(submission.UserID(userID), submission.CreatedAtGT(now.Add(-limit.Window))).
		Order(ent.Asc(submission.FieldCreatedAt)).
		Limit(limit.Max).
		Select(submission.FieldCreatedAt).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("count recent submissions: %w", err)
	}
	if len(recent) < limit.Max {
		return 0, nil
	}
	// The oldest submission in the window leaves it first.
	wait := recent[0].CreatedAt.Add(limit.Window).Sub(now)
	return min(max(wait, time.Second), limit.Window), nil
}

// replaySubmission answers a request whose Idempotency-Key already exists.
func (s *server) replaySubmission(w http.ResponseWriter, r *http.Request, userID, key, hash string) {
	existing, err := s.Store.Ent.Submission.Query().
		Where(submission.UserID(userID), submission.IdempotencyKey(key)).
		WithProblem().
		WithResults(orderResults).
		Only(r.Context())
	if err != nil {
		s.internalError(w, r, fmt.Errorf("load submission for idempotent replay: %w", err))
		return
	}
	if existing.RequestHash != hash {
		s.writeProblem(w, http.StatusConflict, "Idempotency-Key was already used with a different request body")
		return
	}
	w.Header().Set("Location", "/api/submissions/"+existing.ID.String())
	s.writeJSON(w, http.StatusAccepted, toSubmissionJSON(existing))
}

// getSubmission: GET /api/submissions/{id}. Other users' submissions are 404, not 403,
// so IDs cannot be probed.
func (s *server) getSubmission(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.writeProblem(w, http.StatusNotFound, "submission not found")
		return
	}
	sub, err := s.Store.Ent.Submission.Query().
		Where(submission.ID(id), submission.UserID(principalFrom(r.Context()).UserID)).
		WithProblem().
		WithResults(orderResults).
		Only(r.Context())
	if ent.IsNotFound(err) {
		s.writeProblem(w, http.StatusNotFound, "submission not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, toSubmissionJSON(sub))
}

// listMySubmissions: GET /api/users/me/submissions, newest first, keyset pagination.
// Three queries per page at most (submissions, problems, results): N+1 guard in tests.
func (s *server) listMySubmissions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := parseLimit(q)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, err.Error())
		return
	}

	query := s.Store.Ent.Submission.Query().
		Where(submission.UserID(principalFrom(r.Context()).UserID))
	if slug := q.Get("problem"); slug != "" {
		if !slugPattern.MatchString(slug) {
			s.writeProblem(w, http.StatusBadRequest, "problem must be a valid slug")
			return
		}
		query = query.Where(submission.HasProblemWith(problem.Slug(slug)))
	}
	if c := q.Get("cursor"); c != "" {
		createdAt, id, err := decodeSubmissionCursor(c)
		if err != nil {
			s.writeProblem(w, http.StatusBadRequest, err.Error())
			return
		}
		query = query.Where(submission.Or(
			submission.CreatedAtLT(createdAt),
			submission.And(submission.CreatedAtEQ(createdAt), submission.IDLT(id)),
		))
	}

	rows, err := query.
		Order(ent.Desc(submission.FieldCreatedAt), ent.Desc(submission.FieldID)).
		Limit(limit + 1).
		WithProblem().
		WithResults(orderResults).
		All(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	page := submissionPageJSON{Items: []submissionJSON{}}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next := encodeSubmissionCursor(last.CreatedAt, last.ID)
		page.NextCursor = &next
	}
	for _, sub := range rows {
		page.Items = append(page.Items, toSubmissionJSON(sub))
	}
	s.writeJSON(w, http.StatusOK, page)
}

func decodeCreateSubmission(w http.ResponseWriter, r *http.Request) (createSubmissionJSON, error) {
	var req createSubmissionJSON
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return req, errors.New("request body is too large")
		}
		return req, errors.New("request body must be a JSON object with problem, language and source")
	}
	switch {
	case !slugPattern.MatchString(req.Problem) || len(req.Problem) > 64:
		return req, errors.New("problem must be a valid slug")
	case submission.LanguageValidator(submission.Language(req.Language)) != nil:
		return req, errors.New("language is not supported")
	case req.Source == "":
		return req, errors.New("source must not be empty")
	case utf8.RuneCountInString(req.Source) > maxSourceChars:
		return req, fmt.Errorf("source must be at most %d characters", maxSourceChars)
	}
	return req, nil
}

// requestHash identifies the request body for idempotent replays.
func requestHash(req createSubmissionJSON) string {
	sum := sha256.Sum256([]byte(req.Problem + "\x00" + req.Language + "\x00" + req.Source))
	return hex.EncodeToString(sum[:])
}

func orderResults(q *ent.SubmissionResultQuery) {
	q.Order(ent.Asc(submissionresult.FieldOrdinal))
}

func toSubmissionJSON(sub *ent.Submission) submissionJSON {
	out := submissionJSON{
		ID:            sub.ID,
		Language:      string(sub.Language),
		Status:        string(sub.Status),
		RuntimeMS:     sub.RuntimeMs,
		MemoryKB:      sub.MemoryKB,
		CompileOutput: sub.CompileOutput,
		CreatedAt:     sub.CreatedAt,
		JudgedAt:      sub.JudgedAt,
		Cases:         make([]caseResultJSON, 0, len(sub.Edges.Results)),
	}
	if sub.Edges.Problem != nil {
		out.Problem = sub.Edges.Problem.Slug
	}
	if sub.Verdict != nil {
		v := string(*sub.Verdict)
		out.Verdict = &v
	}
	for _, res := range sub.Edges.Results {
		out.Cases = append(out.Cases, caseResultJSON{
			Index:     res.Ordinal,
			Verdict:   string(res.Verdict),
			RuntimeMS: res.RuntimeMs,
			MemoryKB:  res.MemoryKB,
		})
	}
	return out
}
