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
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/retek44/CodeArena/backend/internal/judge"
	"github.com/retek44/CodeArena/backend/internal/problem"
	"github.com/retek44/CodeArena/backend/internal/runner"
)

const defaultRunnerURL = "http://localhost:9000/2015-03-31/functions/function/invocations"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("judge", flag.ContinueOnError)
	fs.SetOutput(stderr)
	problemDir := fs.String("problem", "", "problem directory (problem.json + cases/)")
	srcPath := fs.String("src", "", "solution source file")
	lang := fs.String("lang", "", "language (default: from file extension)")
	runnerURL := fs.String("runner-url", envOr("CODEARENA_RUNNER_URL", defaultRunnerURL), "runner invoke URL")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	timeout := fs.Duration("timeout", 2*time.Minute, "overall timeout")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *problemDir == "" || *srcPath == "" {
		fmt.Fprintln(stderr, "usage: judge -problem DIR -src FILE [-lang python] [-runner-url URL] [-json]")
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

	inv := &runner.HTTPInvoker{URL: *runnerURL, Client: &http.Client{Timeout: *timeout}}
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
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py":
		return "python", nil
	default:
		return "", fmt.Errorf("cannot detect language of %s; use -lang", path)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
