// Command worker consumes submission messages from SQS and judges them.
//
// Configuration (environment):
//
//	DATABASE_URL            PostgreSQL connection string
//	SUBMISSIONS_QUEUE_URL   SQS queue (AWS_ENDPOINT_URL_SQS points to ElasticMQ locally)
//	RUNNER_MODE             http (runner container) or lambda
//	RUNNER_URL              http mode: Lambda Runtime Interface Emulator invoke URL (Python runner)
//	RUNNER_URL_CPP          http mode: invoke URL of the C++ runner (optional)
//	RUNNER_FUNCTION_PYTHON  lambda mode: runner function name or ARN
//	RUNNER_FUNCTION_CPP     lambda mode: C++ runner function (optional)
//	WORKER_CONCURRENCY      submissions judged at once (default 4)
//	WORKER_LEASE            lease and retry delay, e.g. 2m (default 2m; match the queue visibility timeout)
//	WORKER_MAX_ATTEMPTS     judging attempts before IE (default 3)
//	EVENT_BUS_NAME          EventBridge bus for status events (optional; WebSocket push)
//	VALKEY_URL              leaderboard cache (optional; redis:// or rediss://)
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
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/valkey-io/valkey-go"

	"github.com/retek44/CodeArena/backend/internal/contest"
	"github.com/retek44/CodeArena/backend/internal/events"
	"github.com/retek44/CodeArena/backend/internal/queue"
	"github.com/retek44/CodeArena/backend/internal/runner"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/telemetry"
	"github.com/retek44/CodeArena/backend/internal/worker"
)

// Set at build time: -ldflags "-X main.version=...".
var version = "dev"

func main() {
	logger := slog.New(telemetry.NewLogHandler(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(logger); err != nil {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// ECS sends SIGTERM before it stops a task; messages in flight are finished.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := telemetry.Setup(ctx, "codearena-worker", version)
	if err != nil {
		return err
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(flushCtx); err != nil {
			logger.Warn("flush traces", "error", err)
		}
	}()

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
	st, err := store.Open(ctx, dbURL, store.WithTracer(telemetry.NewQueryTracer()))
	if err != nil {
		return err
	}
	defer st.Close()

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	telemetry.InstrumentAWS(&awsCfg)
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
	processor.Leaderboard = &contest.Leaderboard{Queries: st.Queries, Cache: valkeyCache(logger), Logger: logger}
	if bus := os.Getenv("EVENT_BUS_NAME"); bus != "" {
		processor.Events = &events.EventBridge{Client: eventbridge.NewFromConfig(awsCfg), Bus: bus}
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
		client := &http.Client{Timeout: timeout, Transport: telemetry.HTTPTransport(http.DefaultTransport)}
		invokers := runner.ByLanguage{"python": &runner.HTTPInvoker{URL: url, Client: client}}
		if cpp := os.Getenv("RUNNER_URL_CPP"); cpp != "" {
			invokers["cpp"] = &runner.HTTPInvoker{URL: cpp, Client: client}
		}
		return invokers, nil
	case "lambda":
		fn := os.Getenv("RUNNER_FUNCTION_PYTHON")
		if fn == "" {
			return nil, errors.New("RUNNER_FUNCTION_PYTHON is required when RUNNER_MODE=lambda")
		}
		functions := map[string]string{"python": fn}
		if cpp := os.Getenv("RUNNER_FUNCTION_CPP"); cpp != "" {
			functions["cpp"] = cpp
		}
		return &runner.LambdaInvoker{Client: lambda.NewFromConfig(cfg), Functions: functions}, nil
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

// valkeyCache connects to the leaderboard cache (VALKEY_URL). Without it, or when
// Valkey is down at startup, leaderboards are computed in Postgres.
func valkeyCache(logger *slog.Logger) valkey.Client {
	url := os.Getenv("VALKEY_URL")
	if url == "" {
		return nil
	}
	client, err := contest.NewCache(url)
	if err != nil {
		logger.Warn("valkey unavailable: leaderboards come from Postgres", "error", err)
		return nil
	}
	return client
}
