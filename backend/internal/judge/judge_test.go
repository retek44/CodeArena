package judge_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/retek44/CodeArena/backend/internal/judge"
	"github.com/retek44/CodeArena/backend/internal/problem"
	"github.com/retek44/CodeArena/backend/internal/protocol"
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

func TestEvaluateFloatChecker(t *testing.T) {
	t.Parallel()

	p := *testProblem
	p.Checker = problem.CheckerFloat
	p.Tolerance = 1e-4
	p.Cases = []problem.Case{{Name: "01", Input: "1\n", Expected: "3.14159\n"}}

	for stdout, want := range map[string]judge.Verdict{
		"3.1416\n": judge.Accepted,
		"3.14\n":   judge.WrongAnswer,
	} {
		resp := protocol.Response{
			Compile: protocol.Compile{Status: protocol.CompileOK},
			Results: []protocol.CaseResult{{ID: "01", Status: protocol.StatusOK, Stdout: stdout}},
		}
		got, err := judge.Evaluate(&p, resp)
		if err != nil || got.Verdict != want {
			t.Errorf("Evaluate(%q) = %s, %v; want %s", stdout, got.Verdict, err, want)
		}
	}
}

// scripted answers each Invoke call with the next response.
type scripted struct {
	reqs  []protocol.Request
	resps []protocol.Response
}

func (s *scripted) Invoke(_ context.Context, req protocol.Request) (protocol.Response, error) {
	s.reqs = append(s.reqs, req)
	if len(s.resps) == 0 {
		return protocol.Response{}, errors.New("unexpected invoke")
	}
	resp := s.resps[0]
	s.resps = s.resps[1:]
	return resp, nil
}

func customProblem() *problem.Problem {
	p := *testProblem
	p.Checker = problem.CheckerCustom
	p.CheckerSource = "import json, sys\nprint('AC')\n"
	return &p
}

func TestRunCustomChecker(t *testing.T) {
	t.Parallel()

	checkerResp := protocol.Response{
		Compile: protocol.Compile{Status: protocol.CompileOK},
		Results: []protocol.CaseResult{
			{ID: "01", Status: protocol.StatusOK, Stdout: "AC\n"},
			{ID: "02", Status: protocol.StatusOK, Stdout: "WA\nexpected 4\n"},
		},
	}
	inv := &scripted{resps: []protocol.Response{response(ok("3\n"), ok("1 3\n")), checkerResp}}
	sub := judge.Submission{ID: "s1", Language: "python", Source: "print()"}

	got, err := judge.Run(t.Context(), inv, customProblem(), sub)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Verdict != judge.WrongAnswer || got.Cases[0].Verdict != judge.Accepted || got.Cases[1].Verdict != judge.WrongAnswer {
		t.Errorf("result = %+v, want AC then WA", got)
	}

	if len(inv.reqs) != 2 {
		t.Fatalf("%d invocations, want submission then checker", len(inv.reqs))
	}
	if inv.reqs[0].Source != "print()" {
		t.Errorf("first invocation must run the submission")
	}
	chk := inv.reqs[1]
	if chk.Source != customProblem().CheckerSource || chk.StopOnFailure || chk.SubmissionID != "s1-checker" {
		t.Errorf("checker request = %+v", chk)
	}
	if chk.Limits.TimeMS != judge.CheckerTimeMS || chk.Limits.MemoryMB != judge.CheckerMemoryMB {
		t.Errorf("checker limits = %+v", chk.Limits)
	}
	var in judge.CheckerInput
	if err := json.Unmarshal([]byte(chk.TestCases[1].Input), &in); err != nil {
		t.Fatal(err)
	}
	if in != (judge.CheckerInput{Input: "2 2\n", Expected: "4\n", Output: "1 3\n"}) {
		t.Errorf("checker input = %+v", in)
	}
}

func TestRunCustomCheckerOnlyChecksSuccessfulRuns(t *testing.T) {
	t.Parallel()

	checkerResp := protocol.Response{
		Compile: protocol.Compile{Status: protocol.CompileOK},
		Results: []protocol.CaseResult{{ID: "01", Status: protocol.StatusOK, Stdout: "AC\n"}},
	}
	inv := &scripted{resps: []protocol.Response{response(ok("3\n"), status(protocol.StatusTimeLimit)), checkerResp}}

	got, err := judge.Run(t.Context(), inv, customProblem(), judge.Submission{ID: "s1"})
	if err != nil || got.Verdict != judge.TimeLimitExceeded {
		t.Fatalf("Run = %+v, %v; want TLE", got, err)
	}
	if n := len(inv.reqs[1].TestCases); n != 1 {
		t.Errorf("checker got %d cases, want only the one that ran ok", n)
	}
}

func TestRunCustomCheckerSkippedOnCompileError(t *testing.T) {
	t.Parallel()

	inv := &scripted{resps: []protocol.Response{{Compile: protocol.Compile{Status: protocol.CompileError, Output: "SyntaxError"}}}}

	got, err := judge.Run(t.Context(), inv, customProblem(), judge.Submission{ID: "s1"})
	if err != nil || got.Verdict != judge.CompileError || len(inv.reqs) != 1 {
		t.Fatalf("Run = %+v, %v after %d invocations; want CE without the checker", got, err, len(inv.reqs))
	}
}

func TestRunBrokenCheckerIsAnError(t *testing.T) {
	t.Parallel()

	broken := map[string]protocol.Response{
		"does not compile": {Compile: protocol.Compile{Status: protocol.CompileError, Output: "SyntaxError"}},
		"crashes": {
			Compile: protocol.Compile{Status: protocol.CompileOK},
			Results: []protocol.CaseResult{{ID: "01", Status: protocol.StatusRuntimeError}, {ID: "02", Status: protocol.StatusOK, Stdout: "AC"}},
		},
		"prints nonsense": {
			Compile: protocol.Compile{Status: protocol.CompileOK},
			Results: []protocol.CaseResult{{ID: "01", Status: protocol.StatusOK, Stdout: "maybe"}, {ID: "02", Status: protocol.StatusOK, Stdout: "AC"}},
		},
		"misses a case": {
			Compile: protocol.Compile{Status: protocol.CompileOK},
			Results: []protocol.CaseResult{{ID: "01", Status: protocol.StatusOK, Stdout: "AC"}},
		},
	}
	for name, checkerResp := range broken {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			inv := &scripted{resps: []protocol.Response{response(ok("3\n"), ok("4\n")), checkerResp}}
			if _, err := judge.Run(t.Context(), inv, customProblem(), judge.Submission{ID: "s1"}); err == nil {
				t.Fatal("expected an error (the worker retries, then IE)")
			}
		})
	}
}

func TestEvaluateRefusesCustomChecker(t *testing.T) {
	t.Parallel()

	if _, err := judge.Evaluate(customProblem(), response(ok("3\n"), ok("4\n"))); err == nil {
		t.Fatal("Evaluate cannot run a custom checker")
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

func TestOutputKBKeepsTheResponseSmall(t *testing.T) {
	t.Parallel()

	for n, want := range map[int]int{0: 1024, 1: 1024, 5: 1024, 6: 853, 100: 51, 10000: 1} {
		if got := judge.OutputKB(n); got != want {
			t.Errorf("OutputKB(%d) = %d, want %d", n, got, want)
		}
		if n > 0 && judge.OutputKB(n)*n > 5120 && judge.OutputKB(n) > 1 {
			t.Errorf("OutputKB(%d) exceeds the 5120 KB budget", n)
		}
	}
}
