package runner_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/retek44/CodeArena/backend/internal/protocol"
	"github.com/retek44/CodeArena/backend/internal/runner"
)

func serve(t *testing.T, status int, body string) *runner.HTTPInvoker {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		var req protocol.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return &runner.HTTPInvoker{URL: srv.URL, Client: srv.Client()}
}

const okBody = `{"protocol_version":1,"submission_id":"s1","compile":{"status":"ok","output":""},
"results":[{"id":"1","status":"ok","cpu_time_ms":5,"wall_time_ms":6,"memory_kb":9000,
"exit_code":0,"signal":null,"stdout":"3\n","stderr":""}],"truncated":false}`

func TestInvokeDecodesResponse(t *testing.T) {
	t.Parallel()

	resp, err := serve(t, http.StatusOK, okBody).Invoke(t.Context(), protocol.Request{SubmissionID: "s1"})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Stdout != "3\n" || *resp.Results[0].ExitCode != 0 {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.Results[0].Signal != nil {
		t.Errorf("signal = %v, want nil", *resp.Results[0].Signal)
	}
}

func TestInvokeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"function error", http.StatusOK, `{"errorType":"InvalidRequestError","errorMessage":"bad"}`, "InvalidRequestError"},
		{"http error", http.StatusInternalServerError, "oops", "HTTP 500"},
		{"not json", http.StatusOK, "nope", "decode"},
		{"unknown field", http.StatusOK, strings.Replace(okBody, `"truncated"`, `"surprise":1,"truncated"`, 1), "unknown field"},
		{"wrong protocol version", http.StatusOK, strings.Replace(okBody, `"protocol_version":1`, `"protocol_version":2`, 1), "protocol version"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := serve(t, tc.status, tc.body).Invoke(t.Context(), protocol.Request{})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestInvokeFunctionErrorIsTyped(t *testing.T) {
	t.Parallel()

	_, err := serve(t, http.StatusOK, `{"errorType":"Runtime.ExitError","errorMessage":"exit status 1"}`).
		Invoke(t.Context(), protocol.Request{})

	var fnErr *runner.FunctionError
	if !errors.As(err, &fnErr) || fnErr.Type != "Runtime.ExitError" {
		t.Fatalf("err = %v, want *FunctionError", err)
	}
}
