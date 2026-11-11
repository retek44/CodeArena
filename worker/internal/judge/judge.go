// Package judge turns a runner response into a verdict.
package judge

import (
	"context"
	"fmt"

	"github.com/retek44/CodeArena/worker/internal/checker"
	"github.com/retek44/CodeArena/worker/internal/problem"
	"github.com/retek44/CodeArena/worker/internal/protocol"
	"github.com/retek44/CodeArena/worker/internal/runner"
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
		Limits:          protocol.Limits{TimeMS: p.TimeMS, MemoryMB: p.MemoryMB},
		StopOnFailure:   true,
	}
	for _, c := range p.Cases {
		req.TestCases = append(req.TestCases, protocol.TestCase{ID: c.Name, Input: c.Input})
	}

	resp, err := inv.Invoke(ctx, req)
	if err != nil {
		return Result{}, err
	}
	return Evaluate(p, resp)
}

// Evaluate maps runner statuses to verdicts and checks outputs.
// The submission verdict is the verdict of the first test case that is not AC.
func Evaluate(p *problem.Problem, resp protocol.Response) (Result, error) {
	if resp.Compile.Status != protocol.CompileOK {
		return Result{Verdict: CompileError, CompileOutput: resp.Compile.Output}, nil
	}
	if len(resp.Results) != len(p.Cases) {
		return Result{}, fmt.Errorf("runner returned %d results for %d test cases", len(resp.Results), len(p.Cases))
	}

	result := Result{Verdict: Accepted}
	for i, c := range p.Cases {
		r := resp.Results[i]
		if r.ID != c.Name {
			return Result{}, fmt.Errorf("result %d is for test case %q, want %q", i, r.ID, c.Name)
		}

		v, err := caseVerdict(r, c.Expected)
		if err != nil {
			return Result{}, err
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

func caseVerdict(r protocol.CaseResult, expected string) (Verdict, error) {
	switch r.Status {
	case protocol.StatusOK:
		if checker.Exact(r.Stdout, expected) {
			return Accepted, nil
		}
		return WrongAnswer, nil
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
		return "", fmt.Errorf("unknown runner status %q", r.Status)
	}
}
