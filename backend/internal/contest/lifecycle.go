package contest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/retek44/CodeArena/backend/internal/events"
	"github.com/retek44/CodeArena/backend/internal/store"
	"github.com/retek44/CodeArena/backend/internal/store/ent"
	entcontest "github.com/retek44/CodeArena/backend/internal/store/ent/contest"
)

// Steps of the contest lifecycle. Step Functions runs them as ECS tasks at the
// contest's times (infra/modules/contests); each step is idempotent, so a retry or a
// manual re-run does no harm.
const (
	StepStart    = "start"
	StepFreeze   = "freeze"
	StepEnd      = "end"
	StepFinalize = "finalize"
)

// Mailer sends the result e-mail of one participant.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Directory finds a user's e-mail address (Cognito); "" means none.
type Directory interface {
	Email(ctx context.Context, userID string) (string, error)
}

// Lifecycle runs the steps.
type Lifecycle struct {
	Store       *store.Store
	Leaderboard *Leaderboard
	Events      events.Publisher
	// Mailer and Directory are optional: without them finalize sends no e-mail.
	Mailer    Mailer
	Directory Directory
	Logger    *slog.Logger
	Now       func() time.Time
}

var stepStatus = map[string]entcontest.Status{
	StepStart:    entcontest.StatusRunning,
	StepFreeze:   entcontest.StatusFrozen,
	StepEnd:      entcontest.StatusEnded,
	StepFinalize: entcontest.StatusFinal,
}

// Run executes one step for one contest.
func (l *Lifecycle) Run(ctx context.Context, step string, contestID int) error {
	status, ok := stepStatus[step]
	if !ok {
		return fmt.Errorf("unknown step %q (start, freeze, end, finalize)", step)
	}
	c, err := l.Store.Ent.Contest.Get(ctx, contestID)
	if err != nil {
		return fmt.Errorf("load contest %d: %w", contestID, err)
	}
	times := Times{ID: c.ID, StartsAt: c.StartsAt, FreezeAt: c.FreezeAt, EndsAt: c.EndsAt}
	log := l.Logger.With("contest", c.Slug, "step", step)

	switch step {
	case StepFreeze:
		// The frozen view must not depend on the first reader after the freeze.
		if err := l.Leaderboard.Snapshot(ctx, times); err != nil {
			log.Warn("frozen snapshot", "error", err)
		}
	case StepFinalize:
		if err := l.mailResults(ctx, c, times, log); err != nil {
			return err
		}
	}

	if err := c.Update().SetStatus(status).Exec(ctx); err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	phase := string(times.PhaseAt(l.now()))
	if step == StepFinalize {
		phase = "final"
	}
	if l.Events != nil {
		if err := l.Events.ContestUpdated(ctx, events.DetailTypeContestPhase, events.ContestUpdated{Contest: c.Slug, Phase: phase}); err != nil {
			log.Warn("publish phase", "error", err)
		}
	}
	log.Info("contest step done", "status", status, "phase", phase)
	return nil
}

func (l *Lifecycle) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

// mailResults sends every participant their final rank. Users without an address
// (local dev users) are skipped; a failed e-mail does not stop the others.
func (l *Lifecycle) mailResults(ctx context.Context, c *ent.Contest, times Times, log *slog.Logger) error {
	if l.Mailer == nil || l.Directory == nil {
		log.Info("no mailer configured, result e-mails skipped")
		return nil
	}
	rows, err := l.Leaderboard.compute(ctx, times, times.EndsAt, "")
	if err != nil {
		return err
	}
	sent := 0
	for _, r := range rows {
		to, err := l.Directory.Email(ctx, r.UserID)
		if err != nil || to == "" {
			log.Info("no e-mail address", "user_id", r.UserID, "error", err)
			continue
		}
		subject, body := ResultMail(c.Title, r, len(rows))
		if err := l.Mailer.Send(ctx, to, subject, body); err != nil {
			log.Warn("result e-mail", "user_id", r.UserID, "error", err)
			continue
		}
		sent++
	}
	log.Info("result e-mails sent", "sent", sent, "participants", len(rows))
	return nil
}

// ResultMail is the text of the result e-mail.
func ResultMail(title string, r Row, participants int) (string, string) {
	subject := fmt.Sprintf("%s: you finished #%d", title, r.Rank)
	var b strings.Builder
	fmt.Fprintf(&b, "Hi %s,\n\n", r.Username)
	fmt.Fprintf(&b, "%s has ended. You finished #%d of %d with %d solved and %d penalty minutes.\n\n",
		title, r.Rank, participants, r.Solved, r.Penalty)
	for _, p := range r.Problems {
		if p.SolvedMinute != nil {
			fmt.Fprintf(&b, "  %s  solved at minute %d (%d rejected before)\n", p.Label, *p.SolvedMinute, p.Rejected)
		} else {
			fmt.Fprintf(&b, "  %s  not solved\n", p.Label)
		}
	}
	b.WriteString("\nThanks for taking part.\nCodeArena\n")
	return subject, b.String()
}
