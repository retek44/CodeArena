// Package runner invokes runner functions.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/retek44/CodeArena/backend/internal/protocol"
)

// maxResponseBytes matches the Lambda synchronous response limit.
const maxResponseBytes = 6 << 20

// Invoker runs a submission on a runner.
type Invoker interface {
	Invoke(ctx context.Context, req protocol.Request) (protocol.Response, error)
}

// FunctionError is returned when the runner function itself failed
// (invalid request, crash, recycled execution environment).
type FunctionError struct {
	Type    string `json:"errorType"`
	Message string `json:"errorMessage"`
}

func (e *FunctionError) Error() string {
	return fmt.Sprintf("runner function error %s: %s", e.Type, e.Message)
}

// HTTPInvoker calls the Lambda Runtime Interface Emulator invoke endpoint, for example
// http://localhost:9000/2015-03-31/functions/function/invocations.
// In AWS the worker uses the Lambda Invoke API instead (phase 3).
type HTTPInvoker struct {
	URL    string
	Client *http.Client
}

// Invoke sends the request and decodes the runner response.
func (h *HTTPInvoker) Invoke(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return protocol.Response{}, fmt.Errorf("encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return protocol.Response{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return protocol.Response{}, fmt.Errorf("invoke runner: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return protocol.Response{}, fmt.Errorf("read runner response: %w", err)
	}
	if len(raw) > maxResponseBytes {
		return protocol.Response{}, errors.New("runner response exceeds 6 MB")
	}
	if resp.StatusCode != http.StatusOK {
		return protocol.Response{}, fmt.Errorf("runner returned HTTP %d: %s", resp.StatusCode, truncate(raw, 512))
	}

	return decode(raw)
}

func decode(raw []byte) (protocol.Response, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return protocol.Response{}, fmt.Errorf("decode runner response: %w", err)
	}
	if _, ok := probe["errorType"]; ok {
		fnErr := &FunctionError{}
		if err := json.Unmarshal(raw, fnErr); err != nil {
			return protocol.Response{}, fmt.Errorf("decode runner error: %w", err)
		}
		return protocol.Response{}, fnErr
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var out protocol.Response
	if err := dec.Decode(&out); err != nil {
		return protocol.Response{}, fmt.Errorf("decode runner response: %w", err)
	}
	if out.ProtocolVersion != protocol.Version {
		return protocol.Response{}, fmt.Errorf("runner protocol version %d, want %d", out.ProtocolVersion, protocol.Version)
	}
	return out, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
