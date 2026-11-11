//go:build integration

// Package storetest runs PostgreSQL in a container for integration tests.
//
// One container serves a whole test package (Main). Migrations are applied once to a
// template database; every test gets its own copy (CREATE DATABASE ... TEMPLATE), so
// tests are isolated and fast.
//
// Docker Desktop on Windows: if the Ryuk reaper container fails to start, run with
// TESTCONTAINERS_RYUK_DISABLED=true (Main still terminates the container).
package storetest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/retek44/CodeArena/api/internal/store"
)

const (
	postgresImage = "postgres:18-alpine"
	templateDB    = "codearena_template"
)

var (
	adminURL  string
	dbCounter atomic.Int64
)

// Main starts PostgreSQL, migrates the template database, runs the tests and
// stops the container. Call it from TestMain: os.Exit(storetest.Main(m)).
func Main(m *testing.M) int {
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, postgresImage,
		postgres.WithDatabase(templateDB),
		postgres.WithUsername("codearena"),
		postgres.WithPassword("codearena"),
		postgres.BasicWaitStrategies(),
	)
	if ctr != nil {
		defer func() {
			if err := ctr.Terminate(context.Background()); err != nil {
				log.Printf("terminate postgres: %v", err)
			}
		}()
	}
	if err != nil {
		log.Printf("start postgres: %v (on Docker Desktop try TESTCONTAINERS_RYUK_DISABLED=true)", err)
		return 1
	}

	templateURL, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Printf("connection string: %v", err)
		return 1
	}
	if err := applyMigrations(ctx, templateURL); err != nil {
		log.Print(err)
		return 1
	}
	adminURL = withDatabase(templateURL, "postgres")
	return m.Run()
}

// QueryCounter counts SQL statements; tests use it to assert an upper bound per request.
type QueryCounter struct {
	n atomic.Int64
}

// TraceQueryStart implements pgx.QueryTracer.
func (c *QueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

// TraceQueryEnd implements pgx.QueryTracer.
func (c *QueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// Reset sets the count to zero.
func (c *QueryCounter) Reset() { c.n.Store(0) }

// Count returns the number of statements since the last Reset.
func (c *QueryCounter) Count() int64 { return c.n.Load() }

// New creates a fresh database from the migrated template and opens a Store on it.
func New(t *testing.T) (*store.Store, *QueryCounter) {
	t.Helper()
	ctx := t.Context()
	name := fmt.Sprintf("test_%d", dbCounter.Add(1))

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer admin.Close(context.WithoutCancel(ctx))
	if _, err := admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateDB)); err != nil {
		t.Fatalf("create database: %v", err)
	}

	counter := &QueryCounter{}
	st, err := store.Open(ctx, withDatabase(adminURL, name), store.WithTracer(counter))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		st.Close()
		cleanupCtx := context.Background()
		admin, err := pgx.Connect(cleanupCtx, adminURL)
		if err != nil {
			t.Errorf("connect admin for cleanup: %v", err)
			return
		}
		defer admin.Close(cleanupCtx)
		if _, err := admin.Exec(cleanupCtx, fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", name)); err != nil {
			t.Errorf("drop database: %v", err)
		}
	})
	return st, counter
}

// applyMigrations runs the Atlas migration files in name order, like `atlas migrate apply`.
func applyMigrations(ctx context.Context, databaseURL string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir(), "*.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	if len(files) == 0 {
		return errors.New("no migrations found in api/migrations")
	}
	sort.Strings(files)

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to template: %w", err)
	}
	// The template must have no open connections when tests copy it.
	defer conn.Close(ctx)

	for _, f := range files {
		sqlText, err := os.ReadFile(f) //nolint:gosec // repository migration files
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, string(sqlText)); err != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

// migrationsDir finds api/migrations relative to this source file, so any test
// package can use storetest regardless of its own directory.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
}

func withDatabase(rawURL, name string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	u.Path = "/" + name
	return u.String()
}
