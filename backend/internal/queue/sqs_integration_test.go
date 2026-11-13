//go:build integration

// Tests against ElasticMQ, an SQS-compatible server, in a container.
package queue_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/retek44/CodeArena/backend/internal/outbox"
	"github.com/retek44/CodeArena/backend/internal/queue"
)

const elasticMQImage = "softwaremill/elasticmq-native:1.7.1"

func newSQS(t *testing.T) *sqs.Client {
	t.Helper()
	ctr, err := testcontainers.Run(t.Context(), elasticMQImage,
		testcontainers.WithExposedPorts("9324/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("9324/tcp")),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start elasticmq: %v", err)
	}
	endpoint, err := ctr.PortEndpoint(t.Context(), "9324/tcp", "http")
	if err != nil {
		t.Fatal(err)
	}
	return sqs.New(sqs.Options{
		Region:       "eu-central-1",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
	})
}

func createQueue(t *testing.T, client *sqs.Client, visibility string) string {
	t.Helper()
	out, err := client.CreateQueue(t.Context(), &sqs.CreateQueueInput{
		QueueName:  aws.String("submissions"),
		Attributes: map[string]string{string(types.QueueAttributeNameVisibilityTimeout): visibility},
	})
	if err != nil {
		t.Fatalf("create queue: %v", err)
	}
	return aws.ToString(out.QueueUrl)
}

func TestPublishAndConsumeWithRetry(t *testing.T) {
	client := newSQS(t)
	url := createQueue(t, client, "1")

	pub := &queue.Publisher{Client: client, QueueURLs: map[string]string{outbox.TopicSubmissionCreated: url}}
	if err := pub.Publish(t.Context(), outbox.Message{ID: 7, Topic: outbox.TopicSubmissionCreated, Payload: []byte(`{"v":1}`)}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := pub.Publish(t.Context(), outbox.Message{Topic: "unknown"}); err == nil {
		t.Error("publish to an unconfigured topic succeeded")
	}

	var (
		mu         sync.Mutex
		deliveries []queue.Delivery
		done       = make(chan struct{})
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	consumer := &queue.Consumer{
		Client:   client,
		QueueURL: url,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		// The handler fails once, then succeeds: the message must come back.
		Handler: func(_ context.Context, d queue.Delivery) queue.Decision {
			mu.Lock()
			defer mu.Unlock()
			deliveries = append(deliveries, d)
			if len(deliveries) == 1 {
				return queue.Retry
			}
			close(done)
			return queue.Ack
		},
		RetryDelay: time.Second,
	}
	stopped := make(chan error, 1)
	go func() { stopped <- consumer.Run(ctx) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("message was not redelivered after Retry")
	}
	cancel()
	if err := <-stopped; err != nil {
		t.Errorf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if string(deliveries[0].Body) != `{"v":1}` || deliveries[0].ReceiveCount != 1 || deliveries[1].ReceiveCount != 2 {
		t.Errorf("deliveries = %+v, want the payload twice with receive counts 1 and 2", deliveries)
	}

	// Acked: the queue is empty, even after the visibility timeout.
	time.Sleep(1500 * time.Millisecond)
	out, err := client.ReceiveMessage(t.Context(), &sqs.ReceiveMessageInput{QueueUrl: aws.String(url), WaitTimeSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 0 {
		t.Errorf("%d messages left after Ack, want 0", len(out.Messages))
	}
}
