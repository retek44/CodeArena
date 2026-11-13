package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/retek44/CodeArena/backend/internal/httpapi"
)

func TestRoutes(t *testing.T) {
	t.Parallel()

	handler := httpapi.NewHandler(httpapi.Deps{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Build:  httpapi.BuildInfo{Version: "1.2.3", Commit: "abc123"},
	})

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   map[string]string
	}{
		{
			name:       "health check",
			method:     http.MethodGet,
			path:       "/healthz",
			wantStatus: http.StatusOK,
			wantBody:   map[string]string{"status": "ok"},
		},
		{
			name:       "version",
			method:     http.MethodGet,
			path:       "/version",
			wantStatus: http.StatusOK,
			wantBody:   map[string]string{"version": "1.2.3", "commit": "abc123"},
		},
		{
			name:       "wrong method",
			method:     http.MethodPost,
			path:       "/healthz",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unknown route",
			method:     http.MethodGet,
			path:       "/nope",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody == nil {
				return
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var got map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			for k, want := range tc.wantBody {
				if got[k] != want {
					t.Errorf("body[%q] = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	t.Parallel()

	handler := httpapi.NewHandler(httpapi.Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	for _, path := range []string{"/healthz", "/version", "/nope"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))

		for header, want := range map[string]string{
			"X-Content-Type-Options":       "nosniff",
			"Cross-Origin-Resource-Policy": "same-origin",
			"Cache-Control":                "no-store",
		} {
			if got := rec.Header().Get(header); got != want {
				t.Errorf("%s: %s = %q, want %q", path, header, got, want)
			}
		}
	}
}
