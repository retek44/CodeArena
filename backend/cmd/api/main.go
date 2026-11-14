// Command api runs the CodeArena HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/valkey-io/valkey-go"

	"github.com/retek44/CodeArena/backend/internal/auth"
	"github.com/retek44/CodeArena/backend/internal/contest"
	"github.com/retek44/CodeArena/backend/internal/hint"
	"github.com/retek44/CodeArena/backend/internal/httpapi"
	"github.com/retek44/CodeArena/backend/internal/outbox"
	"github.com/retek44/CodeArena/backend/internal/queue"
	"github.com/retek44/CodeArena/backend/internal/realtime"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/sweeper"
	"github.com/retek44/CodeArena/backend/internal/telemetry"
)

// Set at build time: -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "none"
)

const shutdownTimeout = 20 * time.Second

func main() {
	logger := slog.New(telemetry.NewLogHandler(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// ECS sends SIGTERM before it stops a task.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := telemetry.Setup(ctx, "codearena-api", version)
	if err != nil {
		return err
	}
	defer flushTraces(shutdownTracing, logger)

	deps := httpapi.Deps{Logger: logger, Build: httpapi.BuildInfo{Version: version, Commit: commit}}

	// Without DATABASE_URL only the health routes run (container smoke test).
	if os.Getenv("DATABASE_URL") != "" || os.Getenv("DB_HOST") != "" {
		dbURL, err := store.DatabaseURLFromEnv()
		if err != nil {
			return err
		}
		st, err := store.Open(ctx, dbURL, store.WithTracer(telemetry.NewQueryTracer()))
		if err != nil {
			return err
		}
		defer st.Close()
		deps.Store = st

		auth, err := authenticator(ctx, logger)
		if err != nil {
			return err
		}
		deps.Auth = auth

		hinter, err := newHinter(ctx)
		if err != nil {
			return err
		}
		deps.Hinter = hinter

		tickets, err := newTicketer(ctx)
		if err != nil {
			return err
		}
		deps.Tickets = tickets
		deps.Leaderboard = &contest.Leaderboard{Queries: st.Queries, Cache: valkeyCache(logger), Logger: logger}

		// The outbox relay publishes new submissions to the judge queue (ADR-0011).
		if queueURL := os.Getenv("SUBMISSIONS_QUEUE_URL"); queueURL != "" {
			awsCfg, err := awsConfig(ctx)
			if err != nil {
				return err
			}
			relay := &outbox.Relay{
				Pool:      st.Pool,
				Publisher: &queue.Publisher{Client: sqs.NewFromConfig(awsCfg), QueueURLs: map[string]string{outbox.TopicSubmissionCreated: queueURL}},
				Logger:    logger,
			}
			go relay.Run(ctx)

			// The sweeper re-announces submissions whose queue message is gone (DLQ).
			sweep, err := newSweeper(st, logger)
			if err != nil {
				return err
			}
			go sweep.Run(ctx)
		} else {
			logger.Warn("SUBMISSIONS_QUEUE_URL is not set: submissions stay queued (no outbox relay)")
		}
	} else {
		logger.Warn("no database configured (DATABASE_URL or DB_HOST): serving health routes only")
	}

	addr := net.JoinHostPort("", envOr("PORT", "8080"))
	srv := &http.Server{
		Addr:              addr,
		Handler:           telemetry.HTTPHandler(httpapi.NewHandler(deps)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", addr, "version", version, "commit", commit)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// newHinter selects the hint service: HINT_MODE=fake (local), HINT_FUNCTION (the
// hint Lambda in AWS), or neither (hints are off and the endpoint answers 503).
func newHinter(ctx context.Context) (hint.Hinter, error) {
	if os.Getenv("HINT_MODE") == "fake" {
		return hint.Fake{}, nil
	}
	fn := os.Getenv("HINT_FUNCTION")
	if fn == "" {
		return nil, nil
	}
	cfg, err := awsConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &hint.Lambda{Client: lambda.NewFromConfig(cfg), Function: fn}, nil
}

// newTicketer enables WebSocket tickets when the WebSocket API exists (WS_URL and
// WS_TABLE, set by Terraform). Locally there is none and the SPA polls.
func newTicketer(ctx context.Context) (realtime.Ticketer, error) {
	url, table := os.Getenv("WS_URL"), os.Getenv("WS_TABLE")
	if url == "" || table == "" {
		return nil, nil
	}
	cfg, err := awsConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &realtime.DynamoDB{Client: dynamodb.NewFromConfig(cfg), Table: table, URL: url}, nil
}

// authenticator selects the authentication mode. There is no default: a deployment
// must choose explicitly, so a missing variable cannot turn on the dev mode.
func authenticator(ctx context.Context, logger *slog.Logger) (httpapi.Authenticator, error) {
	switch mode := os.Getenv("AUTH_MODE"); mode {
	case "dev":
		logger.Warn("AUTH_MODE=dev: the X-Dev-User header is trusted; never use this outside local development")
		return httpapi.DevAuthenticator{}, nil
	case "cognito":
		// Values come from the Terraform outputs cognito_issuer and cognito_client_id.
		return auth.NewCognito(ctx, auth.Config{
			Issuer:   os.Getenv("COGNITO_ISSUER"),
			ClientID: os.Getenv("COGNITO_CLIENT_ID"),
		})
	default:
		return nil, fmt.Errorf("AUTH_MODE must be dev or cognito, got %q", mode)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newSweeper reads the sweeper settings (all optional):
//
//	SWEEP_INTERVAL        time between sweeps (default 1m)
//	SWEEP_RUNNING_AFTER   running without progress this long = worker lost (default 10m)
//	SWEEP_QUEUED_AFTER    queued and unclaimed this long = message lost (default 15m)
//	WORKER_MAX_ATTEMPTS   same value as the worker's; exhausted submissions get IE (default 3)
func newSweeper(st *store.Store, logger *slog.Logger) (*sweeper.Sweeper, error) {
	s := &sweeper.Sweeper{Pool: st.Pool, Logger: logger}
	var err error
	if s.Interval, err = durationEnv("SWEEP_INTERVAL", sweeper.DefaultInterval); err != nil {
		return nil, err
	}
	if s.RunningAfter, err = durationEnv("SWEEP_RUNNING_AFTER", sweeper.DefaultRunningAfter); err != nil {
		return nil, err
	}
	if s.QueuedAfter, err = durationEnv("SWEEP_QUEUED_AFTER", sweeper.DefaultQueuedAfter); err != nil {
		return nil, err
	}
	if raw := os.Getenv("WORKER_MAX_ATTEMPTS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, errors.New("WORKER_MAX_ATTEMPTS must be a positive integer")
		}
		s.MaxAttempts = n
	}
	return s, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration such as 10m", key)
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

// awsConfig loads the AWS SDK configuration with tracing of every call.
func awsConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config: %w", err)
	}
	telemetry.InstrumentAWS(&cfg)
	return cfg, nil
}

// flushTraces sends the spans still buffered; ECS gives a stopping task 30 seconds.
func flushTraces(shutdown func(context.Context) error, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		logger.Warn("flush traces", "error", err)
	}
}
