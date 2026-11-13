// Package hint asks the hint service for a nudge after a failed submission.
//
// In AWS the hint Lambda (lambdas/hint) calls Amazon Bedrock with a guardrail that
// blocks full solutions. Locally a fake returns a fixed text, so the flow works
// without AWS. The request never contains hidden test data: only the statement, the
// source, the verdict and, when the failing test case is a sample, its input and
// expected output.
package hint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// Request is the event of the hint Lambda.
type Request struct {
	ProblemTitle string      `json:"problem_title"`
	Statement    string      `json:"statement"`
	Language     string      `json:"language"`
	Source       string      `json:"source"`
	Verdict      string      `json:"verdict"`
	FailedCase   *FailedCase `json:"failed_case,omitempty"`
}

// FailedCase describes the first failing test case. Input and Expected are set only
// for sample test cases, which the user can already see.
type FailedCase struct {
	Ordinal  int    `json:"ordinal"`
	IsSample bool   `json:"is_sample"`
	Input    string `json:"input,omitempty"`
	Expected string `json:"expected,omitempty"`
}

// Status of a hint.
const (
	// StatusOK: the model's answer.
	StatusOK = "ok"
	// StatusBlocked: the guardrail or the code filter replaced the answer.
	StatusBlocked = "blocked"
)

// Response is what the hint Lambda returns.
type Response struct {
	Hint   string `json:"hint"`
	Status string `json:"status"`
	Model  string `json:"model"`
}

// Hinter produces a hint.
type Hinter interface {
	Hint(ctx context.Context, req Request) (Response, error)
}

// LambdaAPI is the subset of the Lambda client the hinter uses.
type LambdaAPI interface {
	Invoke(ctx context.Context, in *lambda.InvokeInput, opts ...func(*lambda.Options)) (*lambda.InvokeOutput, error)
}

// Lambda invokes the hint function synchronously.
type Lambda struct {
	Client   LambdaAPI
	Function string
}

// Hint implements Hinter.
func (l *Lambda) Hint(ctx context.Context, req Request) (Response, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return Response{}, err
	}
	out, err := l.Client.Invoke(ctx, &lambda.InvokeInput{FunctionName: aws.String(l.Function), Payload: payload})
	if err != nil {
		return Response{}, fmt.Errorf("invoke %s: %w", l.Function, err)
	}
	if out.FunctionError != nil {
		return Response{}, fmt.Errorf("hint function error %s: %s", aws.ToString(out.FunctionError), out.Payload)
	}
	var resp Response
	if err := json.Unmarshal(out.Payload, &resp); err != nil {
		return Response{}, fmt.Errorf("decode hint: %w", err)
	}
	if resp.Hint == "" || (resp.Status != StatusOK && resp.Status != StatusBlocked) {
		return Response{}, errors.New("hint function returned no hint")
	}
	return resp, nil
}

// Fake answers with a fixed text (local development and tests: HINT_MODE=fake).
type Fake struct{}

// Hint implements Hinter.
func (Fake) Hint(_ context.Context, req Request) (Response, error) {
	text := fmt.Sprintf("Your %s solution got %s. Re-read the constraints and check the edge cases "+
		"(smallest and largest values, negative numbers, empty input) before the general case.", req.Language, req.Verdict)
	return Response{Hint: text, Status: StatusOK, Model: "fake"}, nil
}
