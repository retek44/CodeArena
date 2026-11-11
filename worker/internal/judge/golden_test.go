package judge_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/retek44/CodeArena/worker/internal/judge"
	"github.com/retek44/CodeArena/worker/internal/problem"
	"github.com/retek44/CodeArena/worker/internal/runner"
)

// TestGolden judges every solution in tests/golden/*/solutions against a running runner
// container and checks that it gets the verdict in its file name (ac.py → AC,
// wa_negative.py → WA). Skipped unless CODEARENA_RUNNER_URL is set.
func TestGolden(t *testing.T) {
	url := os.Getenv("CODEARENA_RUNNER_URL")
	if url == "" {
		t.Skip("CODEARENA_RUNNER_URL not set; start a runner container to run golden tests")
	}

	problemDirs, err := filepath.Glob(filepath.Join("..", "..", "..", "tests", "golden", "*"))
	if err != nil || len(problemDirs) == 0 {
		t.Fatalf("no golden problems found: %v", err)
	}
	inv := &runner.HTTPInvoker{URL: url, Client: &http.Client{Timeout: time.Minute}}

	for _, dir := range problemDirs {
		p, err := problem.Load(dir)
		if err != nil {
			t.Fatalf("load %s: %v", dir, err)
		}
		solutions, _ := filepath.Glob(filepath.Join(dir, "solutions", "*"))
		for _, sol := range solutions {
			name := p.Slug + "/" + filepath.Base(sol)
			want := expectedVerdict(sol)
			t.Run(name, func(t *testing.T) {
				source, err := os.ReadFile(sol) //nolint:gosec // test fixture path from Glob
				if err != nil {
					t.Fatal(err)
				}
				got, err := judge.Run(t.Context(), inv, p, judge.Submission{
					ID:       "golden-" + strings.TrimSuffix(filepath.Base(sol), filepath.Ext(sol)),
					Language: "python",
					Source:   string(source),
				})
				if err != nil {
					t.Fatalf("judge: %v", err)
				}
				if got.Verdict != want {
					t.Errorf("verdict = %s, want %s (cases: %+v)", got.Verdict, want, got.Cases)
				}
			})
		}
	}
}

func expectedVerdict(path string) judge.Verdict {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	prefix, _, _ := strings.Cut(base, "_")
	return judge.Verdict(strings.ToUpper(prefix))
}
