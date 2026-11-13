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
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/retek44/CodeArena/backend/internal/auth"
	"github.com/retek44/CodeArena/backend/internal/httpapi"
	"github.com/retek44/CodeArena/backend/internal/outbox"
	"github.com/retek44/CodeArena/backend/internal/queue"
	"github.com/retek44/CodeArena/backend/internal/store"
)

// Set at build time: -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "none"
)

const shutdownTimeout = 20 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// ECS sends SIGTERM before it stops a task.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deps := httpapi.Deps{Logger: logger, Build: httpapi.BuildInfo{Version: version, Commit: commit}}

	// Without DATABASE_URL only the health routes run (container smoke test).
	if os.Getenv("DATABASE_URL") != "" || os.Getenv("DB_HOST") != "" {
		dbURL, err := store.DatabaseURLFromEnv()
		if err != nil {
			return err
		}
		st, err := store.Open(ctx, dbURL)
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

		// The outbox relay publishes new submissions to the judge queue (ADR-0011).
		if queueURL := os.Getenv("SUBMISSIONS_QUEUE_URL"); queueURL != "" {
			awsCfg, err := config.LoadDefaultConfig(ctx)
			if err != nil {
				return fmt.Errorf("load AWS config: %w", err)
			}
			relay := &outbox.Relay{
				Pool:      st.Pool,
				Publisher: &queue.Publisher{Client: sqs.NewFromConfig(awsCfg), QueueURLs: map[string]string{outbox.TopicSubmissionCreated: queueURL}},
				Logger:    logger,
			}
			go relay.Run(ctx)
		} else {
			logger.Warn("SUBMISSIONS_QUEUE_URL is not set: submissions stay queued (no outbox relay)")
		}
	} else {
		logger.Warn("no database configured (DATABASE_URL or DB_HOST): serving health routes only")
	}

	addr := net.JoinHostPort("", envOr("PORT", "8080"))
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewHandler(deps),
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
