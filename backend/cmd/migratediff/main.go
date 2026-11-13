// Command migratediff generates a versioned migration from the difference between the Ent schema and
// the migrations already in api/migrations, using an empty dev database for replay.
//
//	go run ./cmd/migratediff <name>    (from the api directory)
//
// The dev database comes from docker compose (database atlas_dev). Override with ATLAS_DEV_URL.
package main

import (
	"context"
	"log"
	"os"

	atlas "ariga.io/atlas/sql/migrate"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/lib/pq"

	"github.com/retek44/CodeArena/backend/internal/store/ent/migrate"
)

// Local docker compose credentials, not a secret.
const defaultDevURL = "postgres://codearena:codearena@localhost:5432/atlas_dev?search_path=public&sslmode=disable" //nolint:gosec // local dev database

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("usage: go run ./cmd/migratediff <migration_name>")
	}

	dir, err := atlas.NewLocalDir("migrations")
	if err != nil {
		log.Fatalf("open migrations directory: %v", err)
	}

	devURL := os.Getenv("ATLAS_DEV_URL")
	if devURL == "" {
		devURL = defaultDevURL
	}

	opts := []schema.MigrateOption{
		schema.WithDir(dir),
		schema.WithMigrationMode(schema.ModeReplay),
		schema.WithDialect(dialect.Postgres),
		schema.WithFormatter(atlas.DefaultFormatter),
		// Ent must never drop data on its own; destructive steps are written by hand
		// in a later migration (expand/contract).
		schema.WithDropColumn(false),
		schema.WithDropIndex(true),
	}
	if err := migrate.NamedDiff(context.Background(), devURL, os.Args[1], opts...); err != nil {
		log.Fatalf("generate migration: %v", err)
	}
}
