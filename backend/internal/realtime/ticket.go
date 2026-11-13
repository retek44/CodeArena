// Package realtime issues tickets for the WebSocket API (ADR-0015).
//
// API Gateway WebSocket APIs cannot validate a Cognito JWT on $connect, and browsers
// cannot send an Authorization header with a WebSocket. So the SPA first asks the HTTP
// API (which validates the token as usual) for a ticket: a random, single-use value
// stored in DynamoDB for one minute. The $connect Lambda (lambdas/ws) deletes the
// ticket and binds its user to the connection. A leaked ticket is useless after one
// connection or one minute.
package realtime

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TicketTTL is how long a ticket can be used.
const TicketTTL = time.Minute

// Ticket opens one WebSocket connection.
type Ticket struct {
	Value     string
	URL       string
	ExpiresAt time.Time
}

// Ticketer issues tickets.
type Ticketer interface {
	Issue(ctx context.Context, userID string, now time.Time) (Ticket, error)
}

// PutItemAPI is the subset of the DynamoDB client the ticketer uses.
type PutItemAPI interface {
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

// DynamoDB stores tickets in the connections table of the WebSocket API.
type DynamoDB struct {
	Client PutItemAPI
	Table  string
	// URL is the wss:// endpoint of the WebSocket stage.
	URL string
}

// Issue implements Ticketer.
func (d *DynamoDB) Issue(ctx context.Context, userID string, now time.Time) (Ticket, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Ticket{}, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	expires := now.Add(TicketTTL)
	_, err := d.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.Table),
		Item: map[string]types.AttributeValue{
			"pk":      &types.AttributeValueMemberS{Value: "ticket#" + value},
			"sk":      &types.AttributeValueMemberS{Value: "ticket"},
			"user_id": &types.AttributeValueMemberS{Value: userID},
			// DynamoDB TTL removes the item eventually; the Lambda checks the time itself.
			"expires_at": &types.AttributeValueMemberN{Value: strconv.FormatInt(expires.Unix(), 10)},
		},
		ConditionExpression: aws.String("attribute_not_exists(pk)"),
	})
	if err != nil {
		return Ticket{}, fmt.Errorf("store ticket: %w", err)
	}
	return Ticket{Value: value, URL: d.URL, ExpiresAt: expires}, nil
}
