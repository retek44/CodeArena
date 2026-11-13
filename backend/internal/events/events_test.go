package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/google/uuid"

	"github.com/retek44/CodeArena/backend/internal/events"
)

type fakeBus struct {
	in  *eventbridge.PutEventsInput
	out *eventbridge.PutEventsOutput
	err error
}

func (f *fakeBus) PutEvents(_ context.Context, in *eventbridge.PutEventsInput, _ ...func(*eventbridge.Options)) (*eventbridge.PutEventsOutput, error) {
	f.in = in
	if f.out == nil {
		f.out = &eventbridge.PutEventsOutput{}
	}
	return f.out, f.err
}

func TestEventBridgePublishesTheContract(t *testing.T) {
	t.Parallel()
	bus := &fakeBus{}
	p := &events.EventBridge{Client: bus, Bus: "codearena-dev"}
	id := uuid.New()

	if err := p.SubmissionUpdated(t.Context(), events.SubmissionUpdated{SubmissionID: id, UserID: "alice", Status: "done", Verdict: "WA"}); err != nil {
		t.Fatal(err)
	}
	entry := bus.in.Entries[0]
	if aws.ToString(entry.EventBusName) != "codearena-dev" || aws.ToString(entry.Source) != events.Source ||
		aws.ToString(entry.DetailType) != events.DetailTypeSubmissionUpdated {
		t.Errorf("envelope = %+v", entry)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(aws.ToString(entry.Detail)), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["v"] != float64(1) || detail["submission_id"] != id.String() || detail["user_id"] != "alice" ||
		detail["status"] != "done" || detail["verdict"] != "WA" || detail["at"] == "" {
		t.Errorf("detail = %v", detail)
	}
}

func TestEventBridgeErrors(t *testing.T) {
	t.Parallel()
	failed := &eventbridge.PutEventsOutput{
		FailedEntryCount: 1,
		Entries:          []types.PutEventsResultEntry{{ErrorCode: aws.String("ThrottlingException")}},
	}
	for name, bus := range map[string]*fakeBus{
		"call fails":  {err: errors.New("network")},
		"entry fails": {out: failed},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := &events.EventBridge{Client: bus, Bus: "b"}
			if err := p.SubmissionUpdated(t.Context(), events.SubmissionUpdated{SubmissionID: uuid.New()}); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
