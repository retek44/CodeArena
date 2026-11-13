//go:build integration && pact

package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pact-foundation/pact-go/v2/models"
	"github.com/pact-foundation/pact-go/v2/provider"

	"github.com/retek44/CodeArena/backend/internal/httpapi"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	"github.com/retek44/CodeArena/backend/internal/store/ent/problem"
	"github.com/retek44/CodeArena/backend/internal/store/ent/submission"
)

// TestPactProvider verifies the SPA's consumer contract (frontend/pacts, written by
// `npm run test:pact`) against the real handler and a real database.
//
// Every provider state starts from a fresh database (a copy of the migrated template),
// so interactions cannot affect each other. The SPA sends a Cognito bearer token; the
// request filter replaces it with the development user header for user alice.
//
// Needs cgo and the Pact FFI library (pact-go install), so it has its own build tag:
//
//	go test -tags integration,pact -run TestPactProvider ./internal/httpapi
func TestPactProvider(t *testing.T) {
	var (
		mu      sync.Mutex
		current *api
	)
	fresh := func() *api {
		a := newAPIWith(t, func(d *httpapi.Deps) { d.Hinter = &recordingHinter{} })
		mu.Lock()
		current = a
		mu.Unlock()
		return a
	}
	fresh()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		h := current.handler
		mu.Unlock()
		h.ServeHTTP(w, r)
	}))
	defer server.Close()

	withProblem := func() (*api, *ent.Problem) {
		a := fresh()
		p := a.problem("a-plus-b", problem.DifficultyEasy, true)
		a.testCase(p, 1, 1, true)
		a.testCase(p, 1, 2, false)
		return a, p
	}
	// A running contest weekly-1 (started 5 minutes ago) with one problem: A = a-plus-b.
	withContest := func() *api {
		a, p := withProblem()
		ctx := t.Context()
		start := time.Now().Add(-5 * time.Minute).Truncate(time.Second)
		c := a.store.Ent.Contest.Create().SetSlug("weekly-1").SetTitle("Weekly 1").SetDescriptionMd("One problem.").
			SetStartsAt(start).SetFreezeAt(start.Add(100 * time.Minute)).SetEndsAt(start.Add(2 * time.Hour)).SaveX(ctx)
		a.store.Ent.ContestProblem.Create().SetContestID(c.ID).SetProblemID(p.ID).SetLabel("A").ExecX(ctx)
		return a
	}
	register := func(a *api) {
		rec := a.do(request{method: http.MethodPost, path: "/api/contests/weekly-1/register", user: "alice"})
		expectStatus(t, rec, http.StatusNoContent)
	}
	none := models.ProviderStateResponse{}

	states := models.StateHandlers{
		"problem a-plus-b exists": func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			if setup {
				withProblem()
			}
			return none, nil
		},
		"alice has a wrong-answer submission of a-plus-b": func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			if !setup {
				return none, nil
			}
			a, _ := withProblem()
			s := a.judged("alice", "pact-wrong-answer", submission.VerdictWA, 2)
			s.Update().SetRuntimeMs(12).SetMemoryKB(9000).ExecX(t.Context())
			return models.ProviderStateResponse{"submissionId": s.ID.String()}, nil
		},
		"contest weekly-1 is running": func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			if setup {
				withContest()
			}
			return none, nil
		},
		"alice is registered for the running contest weekly-1": func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			if setup {
				register(withContest())
			}
			return none, nil
		},
		"alice solved problem A of the running contest weekly-1": func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			if !setup {
				return none, nil
			}
			a := withContest()
			register(a)
			rec := a.do(request{method: http.MethodPost, path: "/api/submissions", user: "alice", key: "pact-contest-ac",
				body: map[string]any{"problem": "a-plus-b", "language": "python", "source": "print(3)", "contest": "weekly-1"}})
			expectStatus(t, rec, http.StatusAccepted)
			s := a.store.Ent.Submission.Query().Where(submission.UserID("dev-alice")).OnlyX(t.Context())
			s.Update().SetStatus(submission.StatusDone).SetVerdict(submission.VerdictAC).ExecX(t.Context())
			return none, nil
		},
	}

	// The SPA authenticates with a Cognito token; the test handler uses development users.
	devAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				r.Header.Del("Authorization")
				r.Header.Set(httpapi.DevUserHeader, "alice")
			}
			next.ServeHTTP(w, r)
		})
	}

	err := provider.NewVerifier().VerifyProvider(t, provider.VerifyRequest{
		Provider:             "codearena-api",
		ProviderBaseURL:      server.URL,
		PactFiles:            []string{filepath.Join("..", "..", "..", "frontend", "pacts", "codearena-spa-codearena-api.json")},
		StateHandlers:        states,
		RequestFilter:        devAuth,
		FailIfNoPactsFound:   true,
		DisableColoredOutput: true,
	})
	if err != nil {
		t.Fatal(err)
	}
}
