// Package events publishes domain events of the judge to Amazon EventBridge
// (contracts/asyncapi.yaml). The push Lambda (lambdas/ws) forwards them to the
// browsers that watch a submission over WebSocket.
//
// Publishing is best effort: the verdict is already stored when the event goes out,
// and the SPA keeps polling slowly while a socket is open, so a lost event only
// delays the update by a few seconds.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel/propagation"
)

// EventBridge envelope of the judge's events.
const (
	Source                      = "codearena.judge"
	DetailTypeSubmissionUpdated = "Submission updated"
	DetailTypeLeaderboard       = "Leaderboard updated"
	DetailTypeContestPhase      = "Contest phase changed"
)

// SubmissionUpdated says that a submission changed its status (running, done, failed).
type SubmissionUpdated struct {
	Version      int       `json:"v"`
	SubmissionID uuid.UUID `json:"submission_id"`
	UserID       string    `json:"user_id"`
	Status       string    `json:"status"`
	Verdict      string    `json:"verdict,omitempty"`
	At           time.Time `json:"at"`
}

// ContestUpdated says that a contest's leaderboard changed (DetailTypeLeaderboard) or
// that the contest entered a new phase (DetailTypeContestPhase, Phase set).
type ContestUpdated struct {
	Version int       `json:"v"`
	Contest string    `json:"contest"`
	Phase   string    `json:"phase,omitempty"`
	At      time.Time `json:"at"`
}

// Publisher sends events.
type Publisher interface {
	SubmissionUpdated(ctx context.Context, e SubmissionUpdated) error
	ContestUpdated(ctx context.Context, detailType string, e ContestUpdated) error
}

// Nop drops every event (local development without EventBridge).
type Nop struct{}

// SubmissionUpdated implements Publisher.
func (Nop) SubmissionUpdated(context.Context, SubmissionUpdated) error { return nil }

// ContestUpdated implements Publisher.
func (Nop) ContestUpdated(context.Context, string, ContestUpdated) error { return nil }

// PutEventsAPI is the subset of the EventBridge client the publisher uses.
type PutEventsAPI interface {
	PutEvents(ctx context.Context, in *eventbridge.PutEventsInput, opts ...func(*eventbridge.Options)) (*eventbridge.PutEventsOutput, error)
}

// EventBridge publishes to one event bus.
type EventBridge struct {
	Client PutEventsAPI
	Bus    string
}

// SubmissionUpdated implements Publisher.
func (p *EventBridge) SubmissionUpdated(ctx context.Context, e SubmissionUpdated) error {
	e.Version = 1
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	return p.put(ctx, DetailTypeSubmissionUpdated, e.At, e)
}

// ContestUpdated implements Publisher.
func (p *EventBridge) ContestUpdated(ctx context.Context, detailType string, e ContestUpdated) error {
	e.Version = 1
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	return p.put(ctx, detailType, e.At, e)
}

func (p *EventBridge) put(ctx context.Context, detailType string, at time.Time, e any) error {
	detail, err := json.Marshal(e)
	if err != nil {
		return err
	}
	entry := types.PutEventsRequestEntry{
		EventBusName: aws.String(p.Bus),
		Source:       aws.String(Source),
		DetailType:   aws.String(detailType),
		Detail:       aws.String(string(detail)),
		Time:         aws.Time(at),
		Resources:    []string{},
	}
	// The WebSocket Lambda (active tracing) continues the submission's trace (ADR-0018).
	carrier := propagation.MapCarrier{}
	xray.Propagator{}.Inject(ctx, carrier)
	if header := carrier.Get("X-Amzn-Trace-Id"); header != "" {
		entry.TraceHeader = aws.String(header)
	}
	out, err := p.Client.PutEvents(ctx, &eventbridge.PutEventsInput{Entries: []types.PutEventsRequestEntry{entry}})
	if err != nil {
		return fmt.Errorf("put event: %w", err)
	}
	// PutEvents reports per-entry failures in the response, not as an error.
	if out.FailedEntryCount > 0 && len(out.Entries) > 0 {
		return fmt.Errorf("put event: %s %s", aws.ToString(out.Entries[0].ErrorCode), aws.ToString(out.Entries[0].ErrorMessage))
	}
	return nil
}
