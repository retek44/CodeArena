//go:build integration

package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/retek44/CodeArena/backend/internal/outbox"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/ent/outboxmessage"
	"github.com/retek44/CodeArena/backend/internal/store/storetest"
)

func TestMain(m *testing.M) {
	os.Exit(storetest.Main(m))
}

// recorder is a Publisher that remembers what it sent and can fail on demand.
type recorder struct {
	mu      sync.Mutex
	sent    []outbox.Message
	failFor map[int64]bool
	delay   time.Duration
}

func (r *recorder) Publish(_ context.Context, m outbox.Message) error {
	time.Sleep(r.delay)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failFor[m.ID] {
		return errors.New("queue unavailable")
	}
	r.sent = append(r.sent, m)
	return nil
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sent)
}

func addMessages(t *testing.T, st *store.Store, n int) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = uuid.New()
		tx, err := st.Ent.Tx(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if err := outbox.AddSubmissionCreated(t.Context(), tx, ids[i]); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	return ids
}

func newRelay(st *store.Store, pub outbox.Publisher, batch int) *outbox.Relay {
	return &outbox.Relay{
		Pool:      st.Pool,
		Publisher: pub,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		BatchSize: batch,
	}
}

func TestRelayPublishesInOrderAndMarksSent(t *testing.T) {
	st, _ := storetest.New(t)
	ids := addMessages(t, st, 3)
	pub := &recorder{}

	sent, err := newRelay(st, pub, 10).RelayOnce(t.Context())
	if err != nil || sent != 3 {
		t.Fatalf("RelayOnce = %d, %v; want 3", sent, err)
	}
	for i, m := range pub.sent {
		var payload outbox.SubmissionCreated
		if err := json.Unmarshal(m.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if m.Topic != outbox.TopicSubmissionCreated || payload.SubmissionID != ids[i] || payload.Version != 1 {
			t.Errorf("message %d = %s %+v, want %s in insertion order", i, m.Topic, payload, ids[i])
		}
	}

	// Nothing left: a second run sends nothing.
	if sent, err := newRelay(st, pub, 10).RelayOnce(t.Context()); err != nil || sent != 0 {
		t.Errorf("second RelayOnce = %d, %v; want 0", sent, err)
	}
	if n := st.Ent.OutboxMessage.Query().Where(outboxmessage.SentAtIsNil()).CountX(t.Context()); n != 0 {
		t.Errorf("%d messages still pending", n)
	}
}

func TestRelayBacksOffOnPublishFailure(t *testing.T) {
	st, _ := storetest.New(t)
	addMessages(t, st, 2)
	first := st.Ent.OutboxMessage.Query().Order(outboxmessage.ByID()).FirstX(t.Context())
	pub := &recorder{failFor: map[int64]bool{int64(first.ID): true}}

	sent, err := newRelay(st, pub, 10).RelayOnce(t.Context())
	if err != nil || sent != 1 {
		t.Fatalf("RelayOnce = %d, %v; want 1 (one failure does not block the other)", sent, err)
	}

	failed := st.Ent.OutboxMessage.GetX(t.Context(), first.ID)
	if failed.SentAt != nil || failed.Attempts != 1 || failed.LastError == nil {
		t.Fatalf("failed message: sent=%v attempts=%d error=%v", failed.SentAt, failed.Attempts, failed.LastError)
	}
	if !failed.AvailableAt.After(time.Now()) {
		t.Errorf("available_at = %v, want a time in the future (backoff)", failed.AvailableAt)
	}

	// During the backoff the message is not retried.
	pub.failFor = nil
	if sent, _ := newRelay(st, pub, 10).RelayOnce(t.Context()); sent != 0 {
		t.Errorf("sent %d during backoff, want 0", sent)
	}
}

func TestConcurrentRelaysPublishEachMessageOnce(t *testing.T) {
	st, _ := storetest.New(t)
	addMessages(t, st, 20)
	// Slow publishing keeps the row locks held while the other relays look for work.
	pub := &recorder{delay: 5 * time.Millisecond}

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			relay := newRelay(st, pub, 5)
			for {
				sent, err := relay.RelayOnce(context.WithoutCancel(t.Context()))
				if err != nil {
					t.Errorf("RelayOnce: %v", err)
					return
				}
				if sent == 0 {
					return
				}
			}
		}()
	}
	wg.Wait()

	if n := pub.count(); n != 20 {
		t.Fatalf("published %d messages, want 20", n)
	}
	seen := map[int64]bool{}
	for _, m := range pub.sent {
		if seen[m.ID] {
			t.Errorf("message %d published twice", m.ID)
		}
		seen[m.ID] = true
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	cases := map[int]time.Duration{1: 2 * time.Second, 2: 4 * time.Second, 5: 32 * time.Second, 9: 5 * time.Minute, 50: 5 * time.Minute}
	for attempt, want := range cases {
		if got := outbox.Backoff(attempt); got != want {
			t.Errorf("Backoff(%d) = %v, want %v", attempt, got, want)
		}
	}
}
