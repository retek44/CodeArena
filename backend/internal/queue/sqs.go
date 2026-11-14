// Package queue connects the outbox relay and the worker to Amazon SQS
// (ElasticMQ locally: set AWS_ENDPOINT_URL_SQS).
package queue

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/retek44/CodeArena/backend/internal/outbox"
	"github.com/retek44/CodeArena/backend/internal/telemetry"
)

// API is the subset of the SQS client the package uses (fakes in tests).
type API interface {
	SendMessage(ctx context.Context, in *sqs.SendMessageInput, opts ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, opts ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, opts ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, in *sqs.ChangeMessageVisibilityInput, opts ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

// Publisher implements outbox.Publisher: one queue per topic.
type Publisher struct {
	Client    API
	QueueURLs map[string]string
}

// Publish sends one outbox message.
func (p *Publisher) Publish(ctx context.Context, m outbox.Message) error {
	url, ok := p.QueueURLs[m.Topic]
	if !ok {
		return fmt.Errorf("no queue for topic %q", m.Topic)
	}
	attrs := map[string]types.MessageAttributeValue{
		"topic":     {DataType: aws.String("String"), StringValue: aws.String(m.Topic)},
		"outbox_id": {DataType: aws.String("Number"), StringValue: aws.String(strconv.FormatInt(m.ID, 10))},
	}
	// The consumer continues the trace from these attributes (traceparent, X-Amzn-Trace-Id).
	otel.GetTextMapPropagator().Inject(ctx, MessageAttributes(attrs))
	_, err := p.Client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(url),
		MessageBody:       aws.String(string(m.Payload)),
		MessageAttributes: attrs,
	})
	return err
}

// MessageAttributes carries the trace context in SQS message attributes
// (propagation.TextMapCarrier). SQS allows 10 attributes per message.
type MessageAttributes map[string]types.MessageAttributeValue

// Get returns the string value of an attribute.
func (a MessageAttributes) Get(key string) string {
	return aws.ToString(a[key].StringValue)
}

// Set stores a string attribute.
func (a MessageAttributes) Set(key, value string) {
	a[key] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(value)}
}

// Keys lists the attribute names.
func (a MessageAttributes) Keys() []string {
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	return keys
}

// Decision tells the consumer what to do with a message after handling it.
type Decision int

const (
	// Ack deletes the message: it was handled, or it can never be handled.
	Ack Decision = iota
	// Retry keeps the message: SQS delivers it again after RetryDelay. After the
	// queue's maxReceiveCount deliveries it moves to the dead-letter queue.
	Retry
)

// Delivery is one received message.
type Delivery struct {
	Body         []byte
	ReceiveCount int
}

// Handler processes one message.
type Handler func(ctx context.Context, d Delivery) Decision

// Consumer long-polls a queue and runs the handler with bounded concurrency.
type Consumer struct {
	Client      API
	QueueURL    string
	Handler     Handler
	Logger      *slog.Logger
	Concurrency int
	// RetryDelay is the visibility timeout set on a message the handler wants to retry.
	RetryDelay time.Duration
}

// Run consumes until ctx is cancelled. Messages in flight are finished first.
func (c *Consumer) Run(ctx context.Context) error {
	concurrency := max(c.Concurrency, 1)
	slots := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		out, err := c.Client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:                    aws.String(c.QueueURL),
			MaxNumberOfMessages:         int32(min(concurrency, 10)), //nolint:gosec // at most 10
			WaitTimeSeconds:             20,
			MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameApproximateReceiveCount},
			MessageAttributeNames:       otel.GetTextMapPropagator().Fields(),
		})
		if ctx.Err() != nil {
			return nil //nolint:nilerr // shutting down
		}
		if err != nil {
			c.Logger.Error("receive messages", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
			continue
		}

		for _, m := range out.Messages {
			slots <- struct{}{}
			wg.Add(1)
			go func(m types.Message) {
				defer wg.Done()
				defer func() { <-slots }()
				// Finish the message even during shutdown: deleting it avoids a redelivery.
				c.handle(context.WithoutCancel(ctx), m)
			}(m)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, m types.Message) {
	count, _ := strconv.Atoi(m.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)])

	// One span per delivery, a child of the producer's span: the submission stays one trace.
	ctx = otel.GetTextMapPropagator().Extract(ctx, MessageAttributes(m.MessageAttributes))
	queue := path.Base(c.QueueURL)
	ctx, span := telemetry.Tracer().Start(ctx, queue+" process",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			semconv.MessagingSystemAWSSQS,
			semconv.MessagingOperationTypeProcess,
			semconv.MessagingDestinationName(queue),
			semconv.MessagingMessageID(aws.ToString(m.MessageId)),
			attribute.Int("codearena.receive_count", count),
		))
	defer span.End()

	decision := c.Handler(ctx, Delivery{Body: []byte(aws.ToString(m.Body)), ReceiveCount: count})
	if decision == Retry {
		span.SetStatus(codes.Error, "retry")
	}

	switch decision {
	case Ack:
		if _, err := c.Client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(c.QueueURL),
			ReceiptHandle: m.ReceiptHandle,
		}); err != nil {
			c.Logger.Error("delete message", "message_id", aws.ToString(m.MessageId), "error", err)
		}
	case Retry:
		if c.RetryDelay <= 0 {
			return // the queue's visibility timeout applies
		}
		if _, err := c.Client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(c.QueueURL),
			ReceiptHandle:     m.ReceiptHandle,
			VisibilityTimeout: int32(c.RetryDelay.Seconds()),
		}); err != nil {
			c.Logger.Warn("change message visibility", "message_id", aws.ToString(m.MessageId), "error", err)
		}
	default:
		c.Logger.Error("unknown handler decision", "decision", int(decision))
	}
}
