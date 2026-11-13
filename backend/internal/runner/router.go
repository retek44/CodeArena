package runner

import (
	"context"
	"fmt"

	"github.com/retek44/CodeArena/backend/internal/protocol"
)

// ByLanguage sends each request to the invoker of its language: one runner per
// language (one image and one Lambda function each).
type ByLanguage map[string]Invoker

// Invoke implements Invoker.
func (b ByLanguage) Invoke(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	inv, ok := b[req.Language]
	if !ok {
		return protocol.Response{}, fmt.Errorf("no runner for language %q", req.Language)
	}
	return inv.Invoke(ctx, req)
}
