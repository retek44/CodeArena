//go:build integration && pact

package httpapi_test

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/retek44/CodeArena/backend/internal/store/db"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
)

// judged (provider states of the Pact test) submits a solution as user and stores a final verdict for it, as the worker
// would: failing is the ordinal of the first failing test case (0: all accepted).
func (a *api) judged(user, key string, verdict submission.Verdict, failing int) *ent.Submission {
	a.t.Helper()
	ctx := a.t.Context()
	rec := a.do(request{method: http.MethodPost, path: "/api/submissions", user: user, key: key, body: validSubmission()})
	expectStatus(a.t, rec, http.StatusAccepted)
	s := a.store.Ent.Submission.GetX(ctx, uuid.MustParse(decode[submissionBody](a.t, rec).ID))
	ordinals, verdicts := []int64{1}, []string{"AC"}
	if failing > 0 {
		ordinals, verdicts = []int64{1, int64(failing)}, []string{"AC", string(verdict)}
	}
	if failing == 1 {
		ordinals, verdicts = []int64{1}, []string{string(verdict)}
	}
	if err := a.store.Queries.InsertSubmissionResults(ctx, db.InsertSubmissionResultsParams{
		SubmissionID: s.ID,
		Ordinals:     ordinals,
		Verdicts:     verdicts,
		RuntimesMs:   make([]int64, len(ordinals)),
		MemoriesKb:   make([]int64, len(ordinals)),
	}); err != nil {
		a.t.Fatal(err)
	}
	return s.Update().SetStatus(submission.StatusDone).SetVerdict(verdict).SaveX(ctx)
}
