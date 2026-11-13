package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/retek44/CodeArena/backend/internal/protocol"
)

// LambdaAPI is the subset of the Lambda client the invoker uses.
type LambdaAPI interface {
	Invoke(ctx context.Context, in *lambda.InvokeInput, opts ...func(*lambda.Options)) (*lambda.InvokeOutput, error)
}

// LambdaInvoker calls the runner function of the submission's language synchronously.
type LambdaInvoker struct {
	Client LambdaAPI
	// Functions maps a language to its runner function name or ARN.
	Functions map[string]string
}

// Invoke implements Invoker.
func (l *LambdaInvoker) Invoke(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	fn, ok := l.Functions[req.Language]
	if !ok {
		return protocol.Response{}, fmt.Errorf("no runner function for language %q", req.Language)
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return protocol.Response{}, fmt.Errorf("encode request: %w", err)
	}

	out, err := l.Client.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: aws.String(fn),
		Payload:      payload,
	})
	if err != nil {
		return protocol.Response{}, fmt.Errorf("invoke %s: %w", fn, err)
	}
	if out.FunctionError != nil {
		fnErr := &FunctionError{Type: aws.ToString(out.FunctionError)}
		_ = json.Unmarshal(out.Payload, fnErr) // best effort: the payload carries errorType and errorMessage
		return protocol.Response{}, fnErr
	}
	if len(out.Payload) > maxResponseBytes {
		return protocol.Response{}, fmt.Errorf("runner response exceeds 6 MB")
	}
	return decode(out.Payload)
}
