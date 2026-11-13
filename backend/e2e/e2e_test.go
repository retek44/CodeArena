//go:build e2e

// End-to-end test against the running stack (docker compose up -d --build --wait):
// every golden solution goes through API -> outbox -> queue -> worker -> runner ->
// database, and the verdict read back through the API must match the file name.
//
//	go test -tags e2e -count=1 ./e2e/
package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func apiURL() string {
	if u := os.Getenv("CODEARENA_API_URL"); u != "" {
		return strings.TrimSuffix(u, "/")
	}
	return "http://localhost:8080"
}

type submission struct {
	ID      string  `json:"id"`
	Status  string  `json:"status"`
	Verdict *string `json:"verdict"`
	Cases   []struct {
		Verdict string `json:"verdict"`
	} `json:"cases"`
}

var client = &http.Client{Timeout: 10 * time.Second}

// call returns the status code and the body; the response is closed here.
func call(t *testing.T, method, path string, body any, headers map[string]string) (int, []byte) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, apiURL()+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dev-User", "e2e")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, buf.Bytes()
}

func TestGoldenSolutionsEndToEnd(t *testing.T) {
	solutions, err := filepath.Glob(filepath.Join("..", "..", "tests", "golden", "*", "solutions", "*"))
	if err != nil || len(solutions) == 0 {
		t.Fatalf("no golden solutions: %v", err)
	}

	for _, path := range solutions {
		problemSlug := filepath.Base(filepath.Dir(filepath.Dir(path)))
		file := filepath.Base(path)
		want := strings.ToUpper(strings.SplitN(strings.TrimSuffix(file, filepath.Ext(file)), "_", 2)[0])

		t.Run(problemSlug+"/"+file, func(t *testing.T) {
			t.Parallel()
			source, err := os.ReadFile(path) //nolint:gosec // repository fixtures
			if err != nil {
				t.Fatal(err)
			}

			status, body := call(t, http.MethodPost, "/api/submissions",
				map[string]string{"problem": problemSlug, "language": "python", "source": string(source)},
				map[string]string{"Idempotency-Key": uuid.NewString()})
			if status != http.StatusAccepted {
				t.Fatalf("submit: status %d: %s", status, body)
			}
			var created submission
			if err := json.Unmarshal(body, &created); err != nil {
				t.Fatal(err)
			}

			got := waitForVerdict(t, created.ID, 90*time.Second)
			if got.Status != "done" || got.Verdict == nil || *got.Verdict != want {
				t.Fatalf("status=%s verdict=%v, want done %s (cases: %+v)", got.Status, deref(got.Verdict), want, got.Cases)
			}
		})
	}
}

func waitForVerdict(t *testing.T, id string, timeout time.Duration) submission {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		status, body := call(t, http.MethodGet, "/api/submissions/"+id, nil, nil)
		if status != http.StatusOK {
			t.Fatalf("get submission: status %d: %s", status, body)
		}
		var s submission
		if err := json.Unmarshal(body, &s); err != nil {
			t.Fatal(err)
		}
		if s.Status == "done" || s.Status == "failed" {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("no verdict after %s (status %s)", timeout, s.Status)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprint(*s)
}
