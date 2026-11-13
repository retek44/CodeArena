// Command judge judges a local solution against a local problem directory
// using a runner container (Lambda Runtime Interface Emulator).
//
//	judge -problem tests/golden/a-plus-b -src solution.py
//
// Exit code: 0 for AC, 2 for any other verdict, 1 for errors.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/retek44/CodeArena/backend/internal/judge"
	"github.com/retek44/CodeArena/backend/internal/problem"
	"github.com/retek44/CodeArena/backend/internal/runner"
)

const (
	defaultRunnerURL    = "http://localhost:9000/2015-03-31/functions/function/invocations"
	defaultRunnerURLCpp = "http://localhost:9001/2015-03-31/functions/function/invocations"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("judge", flag.ContinueOnError)
	fs.SetOutput(stderr)
	problemDir := fs.String("problem", "", "problem directory (problem.json + cases/)")
	srcPath := fs.String("src", "", "solution source file")
	lang := fs.String("lang", "", "language (default: from file extension)")
	runnerURL := fs.String("runner-url", envOr("CODEARENA_RUNNER_URL", defaultRunnerURL), "Python runner invoke URL (also runs custom checkers)")
	runnerURLCpp := fs.String("runner-url-cpp", envOr("CODEARENA_RUNNER_URL_CPP", defaultRunnerURLCpp), "C++ runner invoke URL")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	timeout := fs.Duration("timeout", 2*time.Minute, "overall timeout")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *problemDir == "" || *srcPath == "" {
		fmt.Fprintln(stderr, "usage: judge -problem DIR -src FILE [-lang python|cpp] [-runner-url URL] [-runner-url-cpp URL] [-json]")
		return 1
	}

	language := *lang
	if language == "" {
		var err error
		if language, err = languageFor(*srcPath); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	p, err := problem.Load(*problemDir)
	if err != nil {
		fmt.Fprintln(stderr, "load problem:", err)
		return 1
	}
	source, err := os.ReadFile(*srcPath) //nolint:gosec // local CLI: the user chooses the file
	if err != nil {
		fmt.Fprintln(stderr, "read source:", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := &http.Client{Timeout: *timeout}
	inv := runner.ByLanguage{
		"python": &runner.HTTPInvoker{URL: *runnerURL, Client: client},
		"cpp":    &runner.HTTPInvoker{URL: *runnerURLCpp, Client: client},
	}
	sub := judge.Submission{ID: "local-" + time.Now().UTC().Format("20060102T150405"), Language: language, Source: string(source)}
	result, err := judge.Run(ctx, inv, p, sub)
	if err != nil {
		fmt.Fprintln(stderr, "judge:", err)
		return 1
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintln(stderr, "encode:", err)
			return 1
		}
	} else {
		printResult(stdout, p, result)
	}

	if result.Verdict != judge.Accepted {
		return 2
	}
	return 0
}

func printResult(w io.Writer, p *problem.Problem, r judge.Result) {
	fmt.Fprintf(w, "Problem: %s   Verdict: %s\n", p.Slug, r.Verdict)
	if r.Verdict == judge.CompileError {
		fmt.Fprintln(w, strings.TrimRight(r.CompileOutput, "\n"))
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CASE\tVERDICT\tCPU ms\tWALL ms\tMEMORY KB")
	for _, c := range r.Cases {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\n", c.Name, c.Verdict, c.CPUTimeMS, c.WallTimeMS, c.MemoryKB)
	}
	_ = tw.Flush()
}

func languageFor(path string) (string, error) {
	if lang := judge.LanguageOf(path); lang != "" {
		return lang, nil
	}
	return "", fmt.Errorf("cannot detect language of %s; use -lang", path)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
