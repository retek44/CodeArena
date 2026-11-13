package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/retek44/CodeArena/backend/internal/hint"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	enthint "github.com/retek44/CodeArena/backend/internal/store/ent/hint"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submissionresult"
	"github.com/retek44/CodeArena/backend/internal/store/ent/testcase"
)

// DefaultHintLimit keeps model cost predictable: 3 new hints per user and hour.
var DefaultHintLimit = RateLimit{Max: 3, Window: time.Hour}

// hintTimeout bounds the model call; it stays below the server's 30 s WriteTimeout.
const hintTimeout = 20 * time.Second

// Verdicts a hint can help with. CE already shows the compiler output; AC needs none.
var hintVerdicts = map[submission.Verdict]bool{
	submission.VerdictWA:  true,
	submission.VerdictRE:  true,
	submission.VerdictTLE: true,
	submission.VerdictMLE: true,
	submission.VerdictOLE: true,
}

type hintJSON struct {
	SubmissionID uuid.UUID `json:"submission_id"`
	Hint         string    `json:"hint"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

func toHintJSON(h *ent.Hint) hintJSON {
	return hintJSON{SubmissionID: h.SubmissionID, Hint: h.Text, Status: string(h.Status), CreatedAt: h.CreatedAt.UTC()}
}

// createHint: POST /api/submissions/{id}/hint. One hint per submission; asking again
// returns the stored hint without a model call and without counting against the limit.
func (s *server) createHint(w http.ResponseWriter, r *http.Request) {
	if s.Hinter == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "hints are not enabled in this environment")
		return
	}
	ctx := r.Context()
	userID := principalFrom(ctx).UserID
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.writeProblem(w, http.StatusNotFound, "submission not found")
		return
	}
	sub, err := s.Store.Ent.Submission.Query().
		Where(submission.ID(id), submission.UserID(userID)).
		WithProblem().
		WithHint().
		WithResults(func(q *ent.SubmissionResultQuery) {
			q.Where(submissionresult.VerdictNEQ(submissionresult.VerdictAC)).
				Order(ent.Asc(submissionresult.FieldOrdinal)).
				Limit(1)
		}).
		Only(ctx)
	if ent.IsNotFound(err) {
		s.writeProblem(w, http.StatusNotFound, "submission not found")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if sub.Edges.Hint != nil {
		s.writeJSON(w, http.StatusOK, toHintJSON(sub.Edges.Hint))
		return
	}
	if sub.Status != submission.StatusDone || sub.Verdict == nil || !hintVerdicts[*sub.Verdict] {
		s.writeProblem(w, http.StatusConflict, "hints are available for judged submissions with WA, RE, TLE, MLE or OLE")
		return
	}

	wait, err := s.hintRetryAfter(ctx, userID)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		s.writeProblem(w, http.StatusTooManyRequests, fmt.Sprintf("at most %d hints per %s", s.HintLimit.Max, s.HintLimit.Window))
		return
	}

	req, err := s.hintRequest(ctx, sub)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	hintCtx, cancel := context.WithTimeout(ctx, hintTimeout)
	defer cancel()
	resp, err := s.Hinter.Hint(hintCtx, req)
	if err != nil {
		s.Logger.Error("hint", "submission_id", sub.ID, "error", err)
		s.writeProblem(w, http.StatusServiceUnavailable, "the hint service is not available, try again later")
		return
	}

	err = s.Store.Ent.Hint.Create().
		SetSubmissionID(sub.ID).
		SetUserID(userID).
		SetText(resp.Hint).
		SetStatus(enthint.Status(resp.Status)).
		SetModel(resp.Model).
		SetCreatedAt(s.Now()).
		OnConflictColumns(enthint.FieldSubmissionID).
		DoNothing().
		Exec(ctx)
	// Two parallel requests for one submission: the first stored hint wins.
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.internalError(w, r, fmt.Errorf("store hint: %w", err))
		return
	}
	stored, err := s.Store.Ent.Hint.Query().Where(enthint.SubmissionID(sub.ID)).Only(ctx)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	s.Logger.Info("hint created", "submission_id", sub.ID, "status", stored.Status, "model", stored.Model)
	s.writeJSON(w, http.StatusOK, toHintJSON(stored))
}

func (s *server) hintRetryAfter(ctx context.Context, userID string) (time.Duration, error) {
	limit := s.HintLimit
	now := s.Now()
	recent, err := s.Store.Ent.Hint.Query().
		Where(enthint.UserID(userID), enthint.CreatedAtGT(now.Add(-limit.Window))).
		Order(ent.Asc(enthint.FieldCreatedAt)).
		Limit(limit.Max).
		Select(enthint.FieldCreatedAt).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("count recent hints: %w", err)
	}
	if len(recent) < limit.Max {
		return 0, nil
	}
	wait := recent[0].CreatedAt.Add(limit.Window).Sub(now)
	return min(max(wait, time.Second), limit.Window), nil
}

// hintRequest builds the model input. Hidden test data never leaves the API: the
// failing case's input and expected output are included only for sample cases.
func (s *server) hintRequest(ctx context.Context, sub *ent.Submission) (hint.Request, error) {
	p := sub.Edges.Problem
	req := hint.Request{
		ProblemTitle: p.Title,
		Statement:    p.StatementMd,
		Language:     string(sub.Language),
		Source:       sub.Source,
		Verdict:      string(*sub.Verdict),
	}
	if len(sub.Edges.Results) == 0 {
		return req, nil
	}
	ordinal := sub.Edges.Results[0].Ordinal
	tc, err := s.Store.Ent.TestCase.Query().
		Where(testcase.ProblemID(p.ID), testcase.ProblemVersion(sub.ProblemVersion), testcase.Ordinal(ordinal)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return req, nil
	}
	if err != nil {
		return req, fmt.Errorf("load failed test case: %w", err)
	}
	req.FailedCase = &hint.FailedCase{Ordinal: ordinal, IsSample: tc.IsSample}
	if tc.IsSample {
		req.FailedCase.Input = tc.Input
		req.FailedCase.Expected = tc.ExpectedOutput
	}
	return req, nil
}
