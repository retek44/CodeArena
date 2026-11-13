// Command seed loads problems from a directory tree (tests/golden layout) into the
// database: problem.json + cases/*.in/*.out per problem. The first case is a sample.
// Existing problems (same slug) are left unchanged, so seeding is safe to repeat.
//
//	DATABASE_URL=... seed -dir tests/golden
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/retek44/CodeArena/backend/internal/problem"
	"github.com/retek44/CodeArena/backend/internal/store"
	entproblem "github.com/retek44/CodeArena/backend/internal/store/ent/problem"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	dir := flag.String("dir", "tests/golden", "directory with one sub-directory per problem")
	flag.Parse()

	if err := run(context.Background(), logger, *dir); err != nil {
		logger.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, dir string) error {
	dbURL, err := store.DatabaseURLFromEnv()
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer st.Close()

	problemDirs, err := filepath.Glob(filepath.Join(dir, "*", "problem.json"))
	if err != nil {
		return err
	}
	for _, path := range problemDirs {
		p, err := problem.Load(filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("load %s: %w", path, err)
		}
		exists, err := st.Ent.Problem.Query().Where(entproblem.Slug(p.Slug)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			logger.Info("problem exists, skipped", "slug", p.Slug)
			continue
		}
		if err := insert(ctx, st, p); err != nil {
			return fmt.Errorf("insert %s: %w", p.Slug, err)
		}
		logger.Info("problem seeded", "slug", p.Slug, "test_cases", len(p.Cases))
	}
	return nil
}

func insert(ctx context.Context, st *store.Store, p *problem.Problem) error {
	tx, err := st.Ent.Tx(ctx)
	if err != nil {
		return err
	}
	title := p.Title
	if title == "" {
		title = p.Slug
	}
	created, err := tx.Problem.Create().
		SetSlug(p.Slug).
		SetTitle(title).
		SetDifficulty(entproblem.DifficultyEasy).
		SetStatementMd("See tests/golden/" + p.Slug + "/README.md").
		SetTimeLimitMs(p.TimeMS).
		SetMemoryLimitMB(p.MemoryMB).
		SetPublished(true).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	for i, c := range p.Cases {
		if err := tx.TestCase.Create().
			SetProblem(created).
			SetProblemVersion(created.Version).
			SetOrdinal(i + 1).
			SetInput(c.Input).
			SetExpectedOutput(c.Expected).
			SetIsSample(i == 0).
			Exec(ctx); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
