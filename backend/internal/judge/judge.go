// Package judge turns a runner response into a verdict.
//
// Outputs are checked in one of three ways (problem.Checker):
//
//   - exact: line-wise comparison in the worker (checker.Exact)
//   - float: numbers within a tolerance, in the worker (checker.Float)
//   - custom: the problem's checker program decides, for problems with more than one
//     correct answer. It runs in the sandbox as a second runner invocation, never
//     together with the submission: the expected output reaches only the checker.
package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/retek44/CodeArena/backend/internal/checker"
	"github.com/retek44/CodeArena/backend/internal/problem"
	"github.com/retek44/CodeArena/backend/internal/protocol"
	"github.com/retek44/CodeArena/backend/internal/runner"
)

// Verdict is the final result of a submission or of one test case.
type Verdict string

// Verdicts.
const (
	Accepted            Verdict = "AC"
	WrongAnswer         Verdict = "WA"
	TimeLimitExceeded   Verdict = "TLE"
	MemoryLimitExceeded Verdict = "MLE"
	OutputLimitExceeded Verdict = "OLE"
	RuntimeError        Verdict = "RE"
	CompileError        Verdict = "CE"
	Skipped             Verdict = "SKIPPED"
	InternalError       Verdict = "IE"
)

// Limits of a custom checker run (per test case). The checker is problem code, not
// user code, but it still runs in the sandbox.
const (
	CheckerTimeMS   = 5000
	CheckerMemoryMB = 256
)

// CheckerInput is the JSON object a custom checker reads from stdin. It prints AC or
// WA on the first line of stdout (anything after that is ignored) and exits with 0.
// Any other result is a broken checker: the attempt fails and the worker retries.
type CheckerInput struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
	Output   string `json:"output"`
}

// Submission is the code to judge.
type Submission struct {
	ID       string
	Language string
	Source   string
}

// CaseVerdict is the verdict of one test case.
type CaseVerdict struct {
	Name       string
	Verdict    Verdict
	CPUTimeMS  int
	MemoryKB   int
	Stderr     string
	WallTimeMS int
}

// Result is the judged submission.
type Result struct {
	Verdict       Verdict
	CompileOutput string
	Cases         []CaseVerdict
}

// Run sends the submission to the runner and judges the outputs.
func Run(ctx context.Context, inv runner.Invoker, p *problem.Problem, sub Submission) (Result, error) {
	req := protocol.Request{
		ProtocolVersion: protocol.Version,
		SubmissionID:    sub.ID,
		Language:        sub.Language,
		Source:          sub.Source,
		Limits:          protocol.Limits{TimeMS: p.TimeMS, MemoryMB: p.MemoryMB, OutputKB: OutputKB(len(p.Cases))},
		StopOnFailure:   true,
	}
	for _, c := range p.Cases {
		req.TestCases = append(req.TestCases, protocol.TestCase{ID: c.Name, Input: c.Input})
	}

	resp, err := inv.Invoke(ctx, req)
	if err != nil {
		return Result{}, err
	}
	if p.Checker != problem.CheckerCustom {
		return Evaluate(p, resp)
	}

	// Custom checker: judge the statuses first, then let the checker decide the
	// test cases that ran successfully.
	if err := checkShape(p, resp); err != nil || resp.Compile.Status != protocol.CompileOK {
		return evaluate(p, resp, nil)
	}
	verdicts, err := runChecker(ctx, inv, p, sub, resp)
	if err != nil {
		return Result{}, err
	}
	return evaluate(p, resp, func(i int, _ string) Verdict { return verdicts[i] })
}

// Evaluate maps runner statuses to verdicts and checks outputs with the exact or
// float checker. The submission verdict is the verdict of the first test case that
// is not AC. Custom checkers need a runner: use Run.
func Evaluate(p *problem.Problem, resp protocol.Response) (Result, error) {
	if p.Checker == problem.CheckerCustom {
		return Result{}, errors.New("a custom checker runs in the sandbox: use Run")
	}
	return evaluate(p, resp, func(i int, stdout string) Verdict {
		if outputMatches(p, stdout, p.Cases[i].Expected) {
			return Accepted
		}
		return WrongAnswer
	})
}

// check returns the verdict of test case i, which ran with status ok.
type check func(i int, stdout string) Verdict

func evaluate(p *problem.Problem, resp protocol.Response, chk check) (Result, error) {
	if resp.Compile.Status != protocol.CompileOK {
		return Result{Verdict: CompileError, CompileOutput: resp.Compile.Output}, nil
	}
	if err := checkShape(p, resp); err != nil {
		return Result{}, err
	}

	result := Result{Verdict: Accepted}
	for i, c := range p.Cases {
		r := resp.Results[i]
		v, err := statusVerdict(r.Status)
		if err != nil {
			return Result{}, err
		}
		if r.Status == protocol.StatusOK {
			v = chk(i, r.Stdout)
		}
		result.Cases = append(result.Cases, CaseVerdict{
			Name:       c.Name,
			Verdict:    v,
			CPUTimeMS:  r.CPUTimeMS,
			WallTimeMS: r.WallTimeMS,
			MemoryKB:   r.MemoryKB,
			Stderr:     r.Stderr,
		})
		if result.Verdict == Accepted && v != Accepted {
			result.Verdict = v
		}
	}

	// Skipped cases without an earlier failure mean the runner ran out of time.
	if result.Verdict == Skipped || (resp.Truncated && result.Verdict == Accepted) {
		result.Verdict = InternalError
	}
	return result, nil
}

// checkShape verifies that the runner answered every test case in order.
func checkShape(p *problem.Problem, resp protocol.Response) error {
	if resp.Compile.Status != protocol.CompileOK {
		return nil
	}
	if len(resp.Results) != len(p.Cases) {
		return fmt.Errorf("runner returned %d results for %d test cases", len(resp.Results), len(p.Cases))
	}
	for i, c := range p.Cases {
		if resp.Results[i].ID != c.Name {
			return fmt.Errorf("result %d is for test case %q, want %q", i, resp.Results[i].ID, c.Name)
		}
	}
	return nil
}

func outputMatches(p *problem.Problem, got, want string) bool {
	if p.Checker == problem.CheckerFloat {
		return checker.Float(got, want, p.Tolerance)
	}
	return checker.Exact(got, want)
}

// statusVerdict maps a runner status to a verdict; ok becomes AC until checked.
func statusVerdict(s protocol.Status) (Verdict, error) {
	switch s {
	case protocol.StatusOK:
		return Accepted, nil
	case protocol.StatusTimeLimit:
		return TimeLimitExceeded, nil
	case protocol.StatusMemoryLimit:
		return MemoryLimitExceeded, nil
	case protocol.StatusOutputLimit:
		return OutputLimitExceeded, nil
	case protocol.StatusRuntimeError:
		return RuntimeError, nil
	case protocol.StatusSkipped:
		return Skipped, nil
	default:
		return "", fmt.Errorf("unknown runner status %q", s)
	}
}

// runChecker runs the custom checker once per test case that ran with status ok and
// returns the verdicts by test case index.
func runChecker(ctx context.Context, inv runner.Invoker, p *problem.Problem, sub Submission, resp protocol.Response) (map[int]Verdict, error) {
	req := protocol.Request{
		ProtocolVersion: protocol.Version,
		SubmissionID:    checkerRunID(sub.ID),
		Language:        "python",
		Source:          p.CheckerSource,
		// A verdict line and a short message are enough.
		Limits:        protocol.Limits{TimeMS: CheckerTimeMS, MemoryMB: CheckerMemoryMB, OutputKB: 4},
		StopOnFailure: false,
	}
	index := map[string]int{}
	for i, c := range p.Cases {
		if resp.Results[i].Status != protocol.StatusOK {
			continue
		}
		in, err := json.Marshal(CheckerInput{Input: c.Input, Expected: c.Expected, Output: resp.Results[i].Stdout})
		if err != nil {
			return nil, err
		}
		req.TestCases = append(req.TestCases, protocol.TestCase{ID: c.Name, Input: string(in)})
		index[c.Name] = i
	}
	verdicts := map[int]Verdict{}
	if len(req.TestCases) == 0 {
		return verdicts, nil
	}

	out, err := inv.Invoke(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("checker: %w", err)
	}
	if out.Compile.Status != protocol.CompileOK {
		return nil, fmt.Errorf("checker does not compile: %s", out.Compile.Output)
	}
	if len(out.Results) != len(req.TestCases) {
		return nil, fmt.Errorf("checker returned %d results for %d test cases", len(out.Results), len(req.TestCases))
	}
	for _, r := range out.Results {
		i, known := index[r.ID]
		if !known {
			return nil, fmt.Errorf("checker returned unknown test case %q", r.ID)
		}
		if r.Status != protocol.StatusOK {
			return nil, fmt.Errorf("checker failed on test case %s: %s %s", r.ID, r.Status, strings.TrimSpace(r.Stderr))
		}
		first, _, _ := strings.Cut(strings.TrimSpace(r.Stdout), "\n")
		switch strings.TrimSpace(first) {
		case "AC":
			verdicts[i] = Accepted
		case "WA":
			verdicts[i] = WrongAnswer
		default:
			return nil, fmt.Errorf("checker printed %q on test case %s, want AC or WA", first, r.ID)
		}
	}
	return verdicts, nil
}

// Output budget of one runner response: the stdout of all test cases together must fit
// into the Lambda response (6 MB), so the runner rejects more than 5120 KB in total.
const (
	maxOutputKB      = 1024
	totalOutputKB    = 5120
	minOutputKBLimit = 1
)

// OutputKB is the per-test-case output limit for n test cases: 1 MB, less when the
// problem has more than five test cases.
func OutputKB(n int) int {
	if n <= 0 {
		return maxOutputKB
	}
	return max(minOutputKBLimit, min(maxOutputKB, totalOutputKB/n))
}

// checkerRunID names the checker run after the submission (runner logs), within the
// protocol's 64 characters.
func checkerRunID(id string) string {
	const suffix = "-checker"
	if len(id) > 64-len(suffix) {
		id = id[:64-len(suffix)]
	}
	return id + suffix
}

// LanguageOf returns the submission language of a source file name, by extension
// (golden solutions, the local judge CLI). Empty if unknown.
func LanguageOf(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".py":
		return "python"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	default:
		return ""
	}
}
