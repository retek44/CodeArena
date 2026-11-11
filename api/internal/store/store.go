// Package store opens the database and exposes both data access layers (ADR-0004):
// Ent for CRUD and sqlc for the hot paths, sharing one pgx connection pool.
package store

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/retek44/CodeArena/api/internal/store/db"
	"github.com/retek44/CodeArena/api/internal/store/ent"
)

// Store bundles the connection pool and both clients.
type Store struct {
	Pool    *pgxpool.Pool
	Ent     *ent.Client
	Queries *db.Queries
}

// Option adjusts the pool configuration.
type Option func(*pgxpool.Config)

// WithTracer installs a pgx query tracer. Every query goes through the pool,
// including Ent's, so tests can count queries per request (N+1 guard).
func WithTracer(tracer pgx.QueryTracer) Option {
	return func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = tracer
	}
}

// Open connects to PostgreSQL. It never runs migrations: those are applied by the
// pipeline before a deploy (expand/contract).
func Open(ctx context.Context, databaseURL string, opts ...Option) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	for _, opt := range opts {
		opt(cfg)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	drv := entsql.OpenDB(dialect.Postgres, stdlib.OpenDBFromPool(pool))
	return &Store{
		Pool:    pool,
		Ent:     ent.NewClient(ent.Driver(drv)),
		Queries: db.New(pool),
	}, nil
}

// Close releases the Ent driver and the pool.
func (s *Store) Close() {
	_ = s.Ent.Close()
	s.Pool.Close()
}
