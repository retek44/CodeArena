package judge_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/retek44/CodeArena/backend/internal/judge"
	"github.com/retek44/CodeArena/backend/internal/problem"
	"github.com/retek44/CodeArena/backend/internal/runner"
)

// TestGolden judges every solution in tests/golden/*/solutions against running runner
// containers and checks that it gets the verdict in its file name (ac.py → AC,
// wa_negative.cpp → WA). The language comes from the extension. Skipped unless
// CODEARENA_RUNNER_URL (Python runner) is set; C++ solutions also need
// CODEARENA_RUNNER_URL_CPP.
func TestGolden(t *testing.T) {
	url := os.Getenv("CODEARENA_RUNNER_URL")
	if url == "" {
		t.Skip("CODEARENA_RUNNER_URL not set; start a runner container to run golden tests")
	}
	urlCpp := os.Getenv("CODEARENA_RUNNER_URL_CPP")

	problemFiles, err := filepath.Glob(filepath.Join("..", "..", "..", "tests", "golden", "*", "problem.json"))
	if err != nil || len(problemFiles) == 0 {
		t.Fatalf("no golden problems found: %v", err)
	}
	client := &http.Client{Timeout: time.Minute}
	inv := runner.ByLanguage{"python": &runner.HTTPInvoker{URL: url, Client: client}}
	if urlCpp != "" {
		inv["cpp"] = &runner.HTTPInvoker{URL: urlCpp, Client: client}
	}

	for _, file := range problemFiles {
		dir := filepath.Dir(file)
		p, err := problem.Load(dir)
		if err != nil {
			t.Fatalf("load %s: %v", dir, err)
		}
		solutions, _ := filepath.Glob(filepath.Join(dir, "solutions", "*"))
		for _, sol := range solutions {
			name := p.Slug + "/" + filepath.Base(sol)
			want := expectedVerdict(sol)
			language := judge.LanguageOf(sol)
			t.Run(name, func(t *testing.T) {
				if _, ok := inv[language]; !ok {
					t.Skipf("no runner for %q (set CODEARENA_RUNNER_URL_CPP for C++)", language)
				}
				source, err := os.ReadFile(sol) //nolint:gosec // test fixture path from Glob
				if err != nil {
					t.Fatal(err)
				}
				got, err := judge.Run(t.Context(), inv, p, judge.Submission{
					ID:       "golden-" + strings.TrimSuffix(filepath.Base(sol), filepath.Ext(sol)),
					Language: language,
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
