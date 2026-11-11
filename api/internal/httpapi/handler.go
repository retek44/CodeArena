// Package httpapi holds the HTTP routes of the CodeArena API.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// BuildInfo describes the running binary. It is set at build time with -ldflags.
type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// NewHandler returns the root handler with all routes registered.
func NewHandler(logger *slog.Logger, build BuildInfo) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, logger, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, logger, http.StatusOK, build)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, logger *slog.Logger, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.Error("write response", "error", err)
	}
}
