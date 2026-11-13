//go:build integration

package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/retek44/CodeArena/backend/internal/httpapi"
	"github.com/retek44/CodeArena/backend/internal/realtime"
)

type fakeTickets struct{ err error }

func (f fakeTickets) Issue(_ context.Context, userID string, now time.Time) (realtime.Ticket, error) {
	if f.err != nil {
		return realtime.Ticket{}, f.err
	}
	return realtime.Ticket{Value: strings.Repeat("t", 43) + userID, URL: "wss://ws.example/live", ExpiresAt: now.Add(time.Minute)}, nil
}

func TestWebSocketTicket(t *testing.T) {
	on := newAPIWith(t, func(d *httpapi.Deps) { d.Tickets = fakeTickets{} })
	rec := on.do(request{method: http.MethodPost, path: "/api/ws/ticket", user: "alice"})
	expectStatus(t, rec, http.StatusOK)
	if got := decode[map[string]string](t, rec); !strings.HasSuffix(got["ticket"], "alice") || got["url"] != "wss://ws.example/live" {
		t.Errorf("ticket = %v", got)
	}
	expectStatus(t, on.do(request{method: http.MethodPost, path: "/api/ws/ticket"}), http.StatusUnauthorized)

	off := newAPI(t)
	expectStatus(t, off.do(request{method: http.MethodPost, path: "/api/ws/ticket", user: "alice"}), http.StatusServiceUnavailable)

	broken := newAPIWith(t, func(d *httpapi.Deps) { d.Tickets = fakeTickets{err: errors.New("throttled")} })
	expectStatus(t, broken.do(request{method: http.MethodPost, path: "/api/ws/ticket", user: "alice"}), http.StatusServiceUnavailable)
}
