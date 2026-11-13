package contest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	entcontest "github.com/retek44/CodeArena/backend/internal/store/ent/contest"
	entproblem "github.com/retek44/CodeArena/backend/internal/store/ent/problem"
)

// Definition is one contest in the seed file (tests/golden/contests.json). Times are
// relative to the first seed run, so a fresh environment always has a contest that
// runs now and one that is scheduled.
type Definition struct {
	Slug               string            `json:"slug"`
	Title              string            `json:"title"`
	Description        string            `json:"description_md"`
	StartsInMinutes    int               `json:"starts_in_minutes"`
	FreezeAfterMinutes int               `json:"freeze_after_minutes"`
	DurationMinutes    int               `json:"duration_minutes"`
	Problems           map[string]string `json:"problems"`
}

// Scheduler arranges the lifecycle steps of a new contest (EventBridge Scheduler and
// Step Functions in AWS; nothing locally, where the phases follow the clock anyway).
type Scheduler interface {
	Schedule(ctx context.Context, c *ent.Contest) error
}

// SeedContests creates the contests of the file that do not exist yet.
func SeedContests(ctx context.Context, st *store.Store, path string, now time.Time, sched Scheduler, logger *slog.Logger) error {
	raw, err := os.ReadFile(path) //nolint:gosec // path from the seed's own flag
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var defs []Definition
	if err := json.Unmarshal(raw, &defs); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	for _, d := range defs {
		if d.FreezeAfterMinutes <= 0 || d.DurationMinutes <= d.FreezeAfterMinutes || len(d.Problems) == 0 {
			return fmt.Errorf("contest %s: need 0 < freeze_after_minutes < duration_minutes and problems", d.Slug)
		}
		exists, err := st.Ent.Contest.Query().Where(entcontest.Slug(d.Slug)).Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			logger.Info("contest exists, skipped", "slug", d.Slug)
			continue
		}
		c, err := createContest(ctx, st, d, now)
		if err != nil {
			return fmt.Errorf("contest %s: %w", d.Slug, err)
		}
		logger.Info("contest seeded", "slug", c.Slug, "starts_at", c.StartsAt)
		if sched != nil {
			if err := sched.Schedule(ctx, c); err != nil {
				return fmt.Errorf("schedule %s: %w", c.Slug, err)
			}
		}
	}
	return nil
}

func createContest(ctx context.Context, st *store.Store, d Definition, now time.Time) (*ent.Contest, error) {
	start := now.Truncate(time.Minute).Add(time.Duration(d.StartsInMinutes) * time.Minute)
	tx, err := st.Ent.Tx(ctx)
	if err != nil {
		return nil, err
	}
	c, err := tx.Contest.Create().
		SetSlug(d.Slug).SetTitle(d.Title).SetDescriptionMd(d.Description).
		SetStartsAt(start).
		SetFreezeAt(start.Add(time.Duration(d.FreezeAfterMinutes) * time.Minute)).
		SetEndsAt(start.Add(time.Duration(d.DurationMinutes) * time.Minute)).
		Save(ctx)
	labels := make([]string, 0, len(d.Problems))
	for label := range d.Problems {
		labels = append(labels, label)
	}
	slices.Sort(labels)
	for _, label := range labels {
		if err != nil {
			break
		}
		var p *ent.Problem
		p, err = tx.Problem.Query().Where(entproblem.Slug(d.Problems[label])).Only(ctx)
		if err == nil {
			err = tx.ContestProblem.Create().SetContestID(c.ID).SetProblemID(p.ID).SetLabel(label).Exec(ctx)
		}
	}
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return c, tx.Commit()
}
