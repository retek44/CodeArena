// Command worker consumes submission messages from SQS and judges them.
//
// Configuration (environment):
//
//	DATABASE_URL            PostgreSQL connection string
//	SUBMISSIONS_QUEUE_URL   SQS queue (AWS_ENDPOINT_URL_SQS points to ElasticMQ locally)
//	RUNNER_MODE             http (runner container) or lambda
//	RUNNER_URL              http mode: Lambda Runtime Interface Emulator invoke URL
//	RUNNER_FUNCTION_PYTHON  lambda mode: runner function name or ARN
//	WORKER_CONCURRENCY      submissions judged at once (default 4)
//	WORKER_LEASE            lease and retry delay, e.g. 2m (default 2m; match the queue visibility timeout)
//	WORKER_MAX_ATTEMPTS     judging attempts before IE (default 3)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/retek44/CodeArena/backend/internal/queue"
	"github.com/retek44/CodeArena/backend/internal/runner"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// ECS sends SIGTERM before it stops a task; messages in flight are finished.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	queueURL := os.Getenv("SUBMISSIONS_QUEUE_URL")
	if queueURL == "" {
		return errors.New("SUBMISSIONS_QUEUE_URL is required")
	}
	lease, err := durationEnv("WORKER_LEASE", 2*time.Minute)
	if err != nil {
		return err
	}
	concurrency, err := intEnv("WORKER_CONCURRENCY", 4)
	if err != nil {
		return err
	}
	maxAttempts, err := intEnv("WORKER_MAX_ATTEMPTS", 3)
	if err != nil {
		return err
	}

	dbURL, err := store.DatabaseURLFromEnv()
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer st.Close()

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	invoker, err := newInvoker(awsCfg, lease)
	if err != nil {
		return err
	}

	processor := &worker.Processor{
		Store:       st,
		Invoker:     invoker,
		Logger:      logger,
		Lease:       lease,
		MaxAttempts: maxAttempts,
	}
	consumer := &queue.Consumer{
		Client:      sqs.NewFromConfig(awsCfg),
		QueueURL:    queueURL,
		Handler:     processor.Handle,
		Logger:      logger,
		Concurrency: concurrency,
		RetryDelay:  lease,
	}

	logger.Info("worker started", "queue", queueURL, "concurrency", concurrency, "lease", lease.String())
	return consumer.Run(ctx)
}

// newInvoker selects how the runner is called: the runner container's emulator
// locally and in CI, the runner Lambda in AWS.
func newInvoker(cfg aws.Config, timeout time.Duration) (runner.Invoker, error) {
	switch mode := os.Getenv("RUNNER_MODE"); mode {
	case "http":
		url := os.Getenv("RUNNER_URL")
		if url == "" {
			return nil, errors.New("RUNNER_URL is required when RUNNER_MODE=http")
		}
		return &runner.HTTPInvoker{URL: url, Client: &http.Client{Timeout: timeout}}, nil
	case "lambda":
		fn := os.Getenv("RUNNER_FUNCTION_PYTHON")
		if fn == "" {
			return nil, errors.New("RUNNER_FUNCTION_PYTHON is required when RUNNER_MODE=lambda")
		}
		return &runner.LambdaInvoker{
			Client:    lambda.NewFromConfig(cfg),
			Functions: map[string]string{"python": fn},
		}, nil
	default:
		return nil, fmt.Errorf("RUNNER_MODE must be http or lambda, got %q", mode)
	}
}

func intEnv(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return n, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration such as 2m", key)
	}
	return d, nil
}
