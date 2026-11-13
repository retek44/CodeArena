package realtime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/retek44/CodeArena/backend/internal/realtime"
)

type fakeTable struct {
	items []*dynamodb.PutItemInput
	err   error
}

func (f *fakeTable) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.items = append(f.items, in)
	return &dynamodb.PutItemOutput{}, f.err
}

func TestIssueStoresASingleUseTicket(t *testing.T) {
	t.Parallel()
	table := &fakeTable{}
	d := &realtime.DynamoDB{Client: table, Table: "codearena-dev-ws", URL: "wss://example/live"}
	now := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

	first, err := d.Issue(t.Context(), "alice", now)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := d.Issue(t.Context(), "alice", now)

	if len(first.Value) < 40 || first.Value == second.Value {
		t.Errorf("tickets %q and %q must be long and random", first.Value, second.Value)
	}
	if first.URL != "wss://example/live" || !first.ExpiresAt.Equal(now.Add(realtime.TicketTTL)) {
		t.Errorf("ticket = %+v", first)
	}
	item := table.items[0]
	if aws.ToString(item.TableName) != "codearena-dev-ws" || aws.ToString(item.ConditionExpression) == "" {
		t.Errorf("put = %+v", item)
	}
	if pk := item.Item["pk"].(*types.AttributeValueMemberS).Value; pk != "ticket#"+first.Value {
		t.Errorf("pk = %s", pk)
	}
	if user := item.Item["user_id"].(*types.AttributeValueMemberS).Value; user != "alice" {
		t.Errorf("user_id = %s", user)
	}
	if exp := item.Item["expires_at"].(*types.AttributeValueMemberN).Value; exp != "1794564060" {
		t.Errorf("expires_at = %s", exp)
	}
}

func TestIssueFails(t *testing.T) {
	t.Parallel()
	d := &realtime.DynamoDB{Client: &fakeTable{err: errors.New("throttled")}, Table: "t"}
	if _, err := d.Issue(t.Context(), "alice", time.Now()); err == nil {
		t.Fatal("expected an error")
	}
}
