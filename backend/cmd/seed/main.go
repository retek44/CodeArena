// Command seed loads problems from a directory tree (tests/golden layout) into the
// database: problem.json, statement.md and cases/*.in/*.out per problem. The first
// case is a sample. Existing problems (same slug) are left unchanged, so seeding is
// safe to repeat; the deploy pipeline runs it on every rollout.
//
// Contests come from <dir>/contests.json with times relative to the first seed. In
// AWS (CONTEST_STATE_MACHINE_ARN, CONTEST_SCHEDULE_GROUP, CONTEST_SCHEDULER_ROLE_ARN)
// each new contest gets its lifecycle: a one-time EventBridge Scheduler schedule
// starts the Step Functions state machine at the start time (ADR-0016).
//
//	DATABASE_URL=... seed -dir tests/golden
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/scheduler"
	schedtypes "github.com/aws/aws-sdk-go-v2/service/scheduler/types"

	"github.com/retek44/CodeArena/backend/internal/contest"
	"github.com/retek44/CodeArena/backend/internal/problem"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
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
	// An empty directory means a broken image or mount, not "nothing to do".
	if len(problemDirs) == 0 {
		return fmt.Errorf("no */problem.json under %s", dir)
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

	sched, err := newScheduler(ctx)
	if err != nil {
		return err
	}
	return contest.SeedContests(ctx, st, filepath.Join(dir, "contests.json"), time.Now(), sched, logger)
}

// newScheduler returns nil locally: the phases follow the clock without it.
func newScheduler(ctx context.Context) (contest.Scheduler, error) {
	machine, group, role := os.Getenv("CONTEST_STATE_MACHINE_ARN"), os.Getenv("CONTEST_SCHEDULE_GROUP"), os.Getenv("CONTEST_SCHEDULER_ROLE_ARN")
	if machine == "" || group == "" || role == "" {
		return nil, nil
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &awsScheduler{
		scheduler: scheduler.NewFromConfig(cfg),
		machine:   machine,
		group:     group,
		role:      role,
	}, nil
}

type awsScheduler struct {
	scheduler *scheduler.Client
	machine   string
	group     string
	role      string
}

// Schedule starts the lifecycle at the contest's start with a one-time schedule that
// deletes itself. A contest that starts now is scheduled two minutes ahead: on a fresh
// environment the seed runs before the state machine exists, and Scheduler retries a
// missing target for up to a day. The phases follow the clock anyway; the lifecycle
// adds events, the frozen snapshot and the result e-mails.
func (a *awsScheduler) Schedule(ctx context.Context, c *ent.Contest) error {
	input, err := json.Marshal(map[string]any{
		"contest_id": c.ID,
		"slug":       c.Slug,
		"freeze_at":  c.FreezeAt.UTC().Format(time.RFC3339),
		"ends_at":    c.EndsAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	at := c.StartsAt
	if earliest := time.Now().Add(2 * time.Minute); at.Before(earliest) {
		at = earliest
	}
	name := fmt.Sprintf("%s-%d", c.Slug, c.StartsAt.Unix())
	_, err = a.scheduler.CreateSchedule(ctx, &scheduler.CreateScheduleInput{
		Name:                       aws.String(name),
		GroupName:                  aws.String(a.group),
		ScheduleExpression:         aws.String("at(" + at.UTC().Format("2006-01-02T15:04:05") + ")"),
		ScheduleExpressionTimezone: aws.String("UTC"),
		FlexibleTimeWindow:         &schedtypes.FlexibleTimeWindow{Mode: schedtypes.FlexibleTimeWindowModeOff},
		ActionAfterCompletion:      schedtypes.ActionAfterCompletionDelete,
		Target: &schedtypes.Target{
			Arn:     aws.String(a.machine),
			RoleArn: aws.String(a.role),
			Input:   aws.String(string(input)),
		},
	})
	var conflict *schedtypes.ConflictException
	if errors.As(err, &conflict) {
		return nil
	}
	return err
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
	statement := p.Statement
	if statement == "" {
		statement = "# " + title
	}
	difficulty := entproblem.DifficultyEasy
	if p.Difficulty != "" {
		difficulty = entproblem.Difficulty(p.Difficulty)
	}
	create := tx.Problem.Create().
		SetSlug(p.Slug).
		SetTitle(title).
		SetDifficulty(difficulty).
		SetStatementMd(statement).
		SetTimeLimitMs(p.TimeMS).
		SetMemoryLimitMB(p.MemoryMB).
		SetChecker(entproblem.Checker(p.Checker)).
		SetPublished(true)
	if p.Checker == problem.CheckerFloat && p.Tolerance > 0 {
		create.SetCheckerTolerance(p.Tolerance)
	}
	if p.Checker == problem.CheckerCustom {
		create.SetCheckerSource(p.CheckerSource)
	}
	created, err := create.Save(ctx)
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
