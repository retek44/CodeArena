package httpapi

import (
	"encoding/json"
	"net/http"
)

// problemDetails is an RFC 9457 error body.
type problemDetails struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func (s *server) writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	body := problemDetails{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: detail}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		s.Logger.Error("write problem response", "error", err)
	}
}

// internalError logs the cause and returns a generic 500; internals never reach the client.
func (s *server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.Logger.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	s.writeProblem(w, http.StatusInternalServerError, "")
}
