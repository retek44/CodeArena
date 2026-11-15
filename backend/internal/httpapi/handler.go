// Package httpapi holds the HTTP routes of the CodeArena API (contracts/openapi.yaml).
package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/retek44/CodeArena/backend/internal/contest"
	"github.com/retek44/CodeArena/backend/internal/realtime"
	"github.com/retek44/CodeArena/backend/internal/store"
)

// BuildInfo describes the running binary. It is set at build time with -ldflags.
type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// Deps are the dependencies of the handler. Store and Auth may be nil: then only
// the health routes are served (useful for container smoke tests).
type Deps struct {
	Logger *slog.Logger
	Build  BuildInfo
	Store  *store.Store
	Auth   Authenticator
	Now    func() time.Time
	// SubmissionLimit caps submissions per user; zero means DefaultSubmissionLimit.
	SubmissionLimit RateLimit
	// GlobalSubmissionLimit caps submissions of all users together, so many accounts
	// cannot turn into unbounded judge cost; zero means DefaultGlobalSubmissionLimit.
	GlobalSubmissionLimit RateLimit
	// Tickets issues WebSocket tickets; nil disables live updates (503, the SPA polls).
	Tickets realtime.Ticketer
	// Leaderboard of contests; nil means Postgres only (no Valkey cache).
	Leaderboard *contest.Leaderboard
}

// RateLimit allows Max events per sliding Window.
type RateLimit struct {
	Max    int
	Window time.Duration
}

// DefaultSubmissionLimit protects the judge from bursts: 10 submissions per minute per user.
var DefaultSubmissionLimit = RateLimit{Max: 10, Window: time.Minute}

// DefaultGlobalSubmissionLimit bounds the judge cost of the whole site: 300 submissions
// per hour from all users together.
var DefaultGlobalSubmissionLimit = RateLimit{Max: 300, Window: time.Hour}

// ParseRateLimit reads "<max>/<window>", for example "300/1h".
func ParseRateLimit(raw string) (RateLimit, error) {
	countText, windowText, ok := strings.Cut(raw, "/")
	if !ok {
		return RateLimit{}, fmt.Errorf("rate limit %q: want <max>/<window>, for example 300/1h", raw)
	}
	count, err := strconv.Atoi(countText)
	if err != nil || count <= 0 {
		return RateLimit{}, fmt.Errorf("rate limit %q: max must be a positive integer", raw)
	}
	window, err := time.ParseDuration(windowText)
	if err != nil || window <= 0 {
		return RateLimit{}, fmt.Errorf("rate limit %q: window must be a positive duration", raw)
	}
	return RateLimit{Max: count, Window: window}, nil
}

// NewHandler returns the root handler with all routes registered.
func NewHandler(d Deps) http.Handler {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.SubmissionLimit.Max <= 0 || d.SubmissionLimit.Window <= 0 {
		d.SubmissionLimit = DefaultSubmissionLimit
	}
	if d.GlobalSubmissionLimit.Max <= 0 || d.GlobalSubmissionLimit.Window <= 0 {
		d.GlobalSubmissionLimit = DefaultGlobalSubmissionLimit
	}
	if d.Leaderboard == nil && d.Store != nil {
		d.Leaderboard = &contest.Leaderboard{Queries: d.Store.Queries, Logger: d.Logger}
	}
	s := &server{Deps: d}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) {
		s.writeJSON(w, http.StatusOK, d.Build)
	})

	if d.Store != nil && d.Auth != nil {
		mux.HandleFunc("GET /api/problems", s.listProblems)
		mux.HandleFunc("GET /api/problems/{slug}", s.getProblem)
		mux.Handle("POST /api/submissions", s.requireAuth(s.createSubmission))
		mux.Handle("GET /api/submissions/{id}", s.requireAuth(s.getSubmission))
		mux.Handle("GET /api/users/me/submissions", s.requireAuth(s.listMySubmissions))
		mux.Handle("POST /api/ws/ticket", s.requireAuth(s.createWSTicket))
		mux.HandleFunc("GET /api/contests", s.listContests)
		mux.HandleFunc("GET /api/contests/{slug}", s.getContest)
		mux.Handle("POST /api/contests/{slug}/register", s.requireAuth(s.registerForContest))
		mux.HandleFunc("GET /api/contests/{slug}/leaderboard", s.getLeaderboard)
	}

	return securityHeaders(mux)
}

// securityHeaders sets headers every API response carries, also when CloudFront is
// bypassed (defense in depth; CloudFront adds CSP and HSTS for browsers). API
// responses are personal and change often, so nothing may store them.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

type server struct {
	Deps
}

func (s *server) writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		s.Logger.Error("write response", "error", err)
	}
}
