package hint_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/retek44/CodeArena/backend/internal/hint"
)

type fakeLambda struct {
	in  *lambda.InvokeInput
	out *lambda.InvokeOutput
	err error
}

func (f *fakeLambda) Invoke(_ context.Context, in *lambda.InvokeInput, _ ...func(*lambda.Options)) (*lambda.InvokeOutput, error) {
	f.in = in
	return f.out, f.err
}

func TestLambdaHinter(t *testing.T) {
	t.Parallel()

	client := &fakeLambda{out: &lambda.InvokeOutput{Payload: []byte(`{"hint":"Check overflow.","status":"ok","model":"m"}`)}}
	h := &hint.Lambda{Client: client, Function: "codearena-dev-hint"}
	req := hint.Request{ProblemTitle: "A + B", Verdict: "WA", FailedCase: &hint.FailedCase{Ordinal: 3}}

	got, err := h.Hint(t.Context(), req)
	if err != nil || got.Hint != "Check overflow." || got.Status != hint.StatusOK {
		t.Fatalf("Hint = %+v, %v", got, err)
	}
	if aws.ToString(client.in.FunctionName) != "codearena-dev-hint" {
		t.Errorf("function = %s", aws.ToString(client.in.FunctionName))
	}
	var sent map[string]any
	if err := json.Unmarshal(client.in.Payload, &sent); err != nil {
		t.Fatal(err)
	}
	failed, _ := sent["failed_case"].(map[string]any)
	if _, leaked := failed["expected"]; leaked {
		t.Error("a hidden test case must not send its expected output")
	}
}

func TestLambdaHinterErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]*fakeLambda{
		"invoke fails":   {err: errors.New("throttled")},
		"function error": {out: &lambda.InvokeOutput{FunctionError: aws.String("Unhandled"), Payload: []byte(`{}`)}},
		"not json":       {out: &lambda.InvokeOutput{Payload: []byte(`nope`)}},
		"empty hint":     {out: &lambda.InvokeOutput{Payload: []byte(`{"hint":"","status":"ok"}`)}},
		"unknown status": {out: &lambda.InvokeOutput{Payload: []byte(`{"hint":"x","status":"maybe"}`)}},
	}
	for name, client := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := (&hint.Lambda{Client: client, Function: "f"}).Hint(t.Context(), hint.Request{}); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
