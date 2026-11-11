package judge_test

import (
	"context"
	"errors"
	"testing"

	"github.com/retek44/CodeArena/worker/internal/judge"
	"github.com/retek44/CodeArena/worker/internal/problem"
	"github.com/retek44/CodeArena/worker/internal/protocol"
)

var testProblem = &problem.Problem{
	Slug:     "a-plus-b",
	TimeMS:   1000,
	MemoryMB: 64,
	Cases: []problem.Case{
		{Name: "01", Input: "1 2\n", Expected: "3\n"},
		{Name: "02", Input: "2 2\n", Expected: "4\n"},
	},
}

func ok(stdout string) protocol.CaseResult {
	return protocol.CaseResult{Status: protocol.StatusOK, Stdout: stdout}
}

func status(s protocol.Status) protocol.CaseResult {
	return protocol.CaseResult{Status: s}
}

func response(results ...protocol.CaseResult) protocol.Response {
	for i := range results {
		results[i].ID = testProblem.Cases[i].Name
	}
	return protocol.Response{
		ProtocolVersion: protocol.Version,
		Compile:         protocol.Compile{Status: protocol.CompileOK},
		Results:         results,
	}
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	truncated := response(ok("3\n"), status(protocol.StatusSkipped))
	truncated.Truncated = true

	tests := []struct {
		name string
		resp protocol.Response
		want judge.Verdict
	}{
		{"all correct", response(ok("3\n"), ok("4\n")), judge.Accepted},
		{"trailing whitespace ignored", response(ok("3  \n\n"), ok("4\r\n")), judge.Accepted},
		{"wrong answer on second case", response(ok("3\n"), ok("5\n")), judge.WrongAnswer},
		{"first failure wins", response(ok("0\n"), status(protocol.StatusTimeLimit)), judge.WrongAnswer},
		{"time limit then skipped", response(status(protocol.StatusTimeLimit), status(protocol.StatusSkipped)), judge.TimeLimitExceeded},
		{"memory limit", response(status(protocol.StatusMemoryLimit), status(protocol.StatusSkipped)), judge.MemoryLimitExceeded},
		{"output limit", response(status(protocol.StatusOutputLimit), status(protocol.StatusSkipped)), judge.OutputLimitExceeded},
		{"runtime error", response(ok("3\n"), status(protocol.StatusRuntimeError)), judge.RuntimeError},
		{"truncated without failure", truncated, judge.InternalError},
		{
			"compile error",
			protocol.Response{Compile: protocol.Compile{Status: protocol.CompileError, Output: "SyntaxError"}},
			judge.CompileError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := judge.Evaluate(testProblem, tc.resp)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if got.Verdict != tc.want {
				t.Errorf("verdict = %s, want %s", got.Verdict, tc.want)
			}
		})
	}
}

func TestEvaluateRejectsInconsistentResponses(t *testing.T) {
	t.Parallel()

	wrongID := response(ok("3\n"), ok("4\n"))
	wrongID.Results[1].ID = "99"

	unknownStatus := response(ok("3\n"), status("exploded"))

	tests := map[string]protocol.Response{
		"missing result": response(ok("3\n")),
		"wrong case id":  wrongID,
		"unknown status": unknownStatus,
	}
	for name, resp := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := judge.Evaluate(testProblem, resp); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

type fakeInvoker struct {
	got  protocol.Request
	resp protocol.Response
	err  error
}

func (f *fakeInvoker) Invoke(_ context.Context, req protocol.Request) (protocol.Response, error) {
	f.got = req
	return f.resp, f.err
}

func TestRunNeverSendsExpectedOutput(t *testing.T) {
	t.Parallel()

	inv := &fakeInvoker{resp: response(ok("3\n"), ok("4\n"))}
	sub := judge.Submission{ID: "s1", Language: "python", Source: "print()"}

	got, err := judge.Run(t.Context(), inv, testProblem, sub)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Verdict != judge.Accepted {
		t.Errorf("verdict = %s, want AC", got.Verdict)
	}

	req := inv.got
	if req.ProtocolVersion != protocol.Version || req.SubmissionID != "s1" || !req.StopOnFailure {
		t.Errorf("unexpected request header: %+v", req)
	}
	if req.Limits.TimeMS != 1000 || req.Limits.MemoryMB != 64 {
		t.Errorf("limits = %+v", req.Limits)
	}
	if len(req.TestCases) != 2 || req.TestCases[0].Input != "1 2\n" {
		t.Errorf("test cases = %+v", req.TestCases)
	}
}

func TestRunReturnsInvokerError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	inv := &fakeInvoker{err: boom}

	if _, err := judge.Run(t.Context(), inv, testProblem, judge.Submission{}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}
