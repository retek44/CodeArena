package events_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/uuid"
	"github.com/pb33f/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/retek44/CodeArena/backend/internal/events"
)

// TestDetailMatchesAsyncAPI checks the published event against contracts/asyncapi.yaml,
// the schema the push Lambda's tests use too: both sides test against one contract.
func TestDetailMatchesAsyncAPI(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "asyncapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	const url = "file:///asyncapi.json"
	c := jsonschema.NewCompiler()
	if err := c.AddResource(url, doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile(url + "#/components/schemas/SubmissionUpdated")
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range []events.SubmissionUpdated{
		{SubmissionID: uuid.New(), UserID: "alice", Status: "running"},
		{SubmissionID: uuid.New(), UserID: "alice", Status: "done", Verdict: "WA"},
		{SubmissionID: uuid.New(), UserID: "alice", Status: "failed", Verdict: "IE"},
	} {
		bus := &fakeBus{}
		if err := (&events.EventBridge{Client: bus, Bus: "b"}).SubmissionUpdated(t.Context(), e); err != nil {
			t.Fatal(err)
		}
		var detail any
		if err := json.Unmarshal([]byte(aws.ToString(bus.in.Entries[0].Detail)), &detail); err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(detail); err != nil {
			t.Errorf("%s: %v", e.Status, err)
		}
	}

	contestSchema, err := c.Compile(url + "#/components/schemas/ContestUpdated")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []events.ContestUpdated{{Contest: "weekly-1"}, {Contest: "weekly-1", Phase: "frozen"}} {
		bus := &fakeBus{}
		if err := (&events.EventBridge{Client: bus, Bus: "b"}).ContestUpdated(t.Context(), events.DetailTypeContestPhase, e); err != nil {
			t.Fatal(err)
		}
		var detail any
		if err := json.Unmarshal([]byte(aws.ToString(bus.in.Entries[0].Detail)), &detail); err != nil {
			t.Fatal(err)
		}
		if err := contestSchema.Validate(detail); err != nil {
			t.Errorf("contest %+v: %v", e, err)
		}
	}

	// The check is not vacuous: an unknown field breaks the contract.
	if err := schema.Validate(map[string]any{"v": 1, "submission_id": "x", "user_id": "u", "status": "done", "at": "t", "extra": 1}); err == nil {
		t.Error("a detail with an unknown field must be rejected")
	}
}
