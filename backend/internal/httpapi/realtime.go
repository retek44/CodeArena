package httpapi

import (
	"net/http"
	"time"
)

type wsTicketJSON struct {
	Ticket    string    `json:"ticket"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// createWSTicket: POST /api/ws/ticket. A single-use ticket for the WebSocket API
// (ADR-0015). Without a WebSocket API (local stack) the SPA gets 503 and polls.
func (s *server) createWSTicket(w http.ResponseWriter, r *http.Request) {
	if s.Tickets == nil {
		s.writeProblem(w, http.StatusServiceUnavailable, "live updates are not enabled in this environment")
		return
	}
	t, err := s.Tickets.Issue(r.Context(), principalFrom(r.Context()).UserID, s.Now())
	if err != nil {
		s.Logger.Error("issue websocket ticket", "error", err)
		s.writeProblem(w, http.StatusServiceUnavailable, "live updates are not available, the page keeps polling")
		return
	}
	s.writeJSON(w, http.StatusOK, wsTicketJSON{Ticket: t.Value, URL: t.URL, ExpiresAt: t.ExpiresAt.UTC()})
}
