package runner_test

import (
	"context"
	"testing"

	"github.com/retek44/CodeArena/backend/internal/protocol"
	"github.com/retek44/CodeArena/backend/internal/runner"
)

type named string

func (n named) Invoke(context.Context, protocol.Request) (protocol.Response, error) {
	return protocol.Response{SubmissionID: string(n)}, nil
}

func TestByLanguageRoutes(t *testing.T) {
	t.Parallel()

	r := runner.ByLanguage{"python": named("py"), "cpp": named("cc")}
	for lang, want := range map[string]string{"python": "py", "cpp": "cc"} {
		resp, err := r.Invoke(t.Context(), protocol.Request{Language: lang})
		if err != nil || resp.SubmissionID != want {
			t.Errorf("%s: got %q, %v; want %q", lang, resp.SubmissionID, err, want)
		}
	}
	if _, err := r.Invoke(t.Context(), protocol.Request{Language: "cobol"}); err == nil {
		t.Error("unknown language must fail")
	}
}
