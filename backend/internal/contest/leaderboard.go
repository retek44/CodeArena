package contest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/valkey-io/valkey-go"

	"github.com/retek44/CodeArena/backend/internal/store/db"
)

// Where a leaderboard came from.
const (
	SourceCache    = "cache"
	SourceDatabase = "database"
)

// liveTTL lets a live cache that missed an update heal itself: after it expires, the
// next read rebuilds it from Postgres.
const liveTTL = 10 * time.Minute

// Board is a leaderboard as served to clients.
type Board struct {
	Rows   []Row
	Frozen bool
	Source string
}

// Leaderboard computes leaderboards in Postgres and caches them in Valkey.
//
// Keys per contest and view (live, frozen); the {id} hash tag keeps both keys of a
// view in one cluster slot, so they change together in MULTI:
//
//	lb:{<id>}:<view>        sorted set, member user ID, score Row.Score
//	lb:{<id>}:<view>:rows   hash, field user ID, value Row as JSON
//
// Valkey is a cache: when it is missing or down, the board comes from Postgres.
type Leaderboard struct {
	Queries *db.Queries
	// Cache may be nil (no Valkey configured).
	Cache  valkey.Client
	Logger *slog.Logger
}

type view struct {
	name   string
	cutoff time.Time
	frozen bool
}

func viewFor(t Times, now time.Time) view {
	if t.PhaseAt(now) == Frozen {
		return view{name: "frozen", cutoff: t.FreezeAt, frozen: true}
	}
	return view{name: "live", cutoff: t.EndsAt}
}

func keys(id int, v string) (string, string) {
	base := "lb:{" + strconv.Itoa(id) + "}:" + v
	return base, base + ":rows"
}

// Board returns the leaderboard visible at now.
func (l *Leaderboard) Board(ctx context.Context, t Times, now time.Time) (Board, error) {
	v := viewFor(t, now)
	if l.Cache != nil {
		rows, found, err := l.read(ctx, t.ID, v.name)
		switch {
		case err != nil:
			l.Logger.Warn("leaderboard cache unavailable, reading Postgres", "contest_id", t.ID, "error", err)
		case found:
			return Board{Rows: rows, Frozen: v.frozen, Source: SourceCache}, nil
		}
	}
	rows, err := l.compute(ctx, t, v.cutoff, "")
	if err != nil {
		return Board{}, err
	}
	if l.Cache != nil && len(rows) > 0 {
		if err := l.write(ctx, t, v, rows); err != nil {
			l.Logger.Warn("fill leaderboard cache", "contest_id", t.ID, "error", err)
		}
	}
	return Board{Rows: rows, Frozen: v.frozen, Source: SourceDatabase}, nil
}

// Record updates one participant after a judged submission or a registration. Only
// existing caches are updated: a missing cache is rebuilt in full by the next read.
func (l *Leaderboard) Record(ctx context.Context, t Times, userID string, now time.Time) error {
	if l.Cache == nil {
		return nil
	}
	views := []view{{name: "live", cutoff: t.EndsAt}}
	if t.PhaseAt(now) == Frozen {
		// A submission made before the freeze may be judged after it.
		views = append(views, view{name: "frozen", cutoff: t.FreezeAt, frozen: true})
	}
	for _, v := range views {
		rows, err := l.compute(ctx, t, v.cutoff, userID)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			continue // not a participant
		}
		if err := l.upsert(ctx, t, v, rows[0]); err != nil {
			return err
		}
	}
	return nil
}

// Snapshot rebuilds the frozen view (the freeze step of the contest lifecycle).
func (l *Leaderboard) Snapshot(ctx context.Context, t Times) error {
	if l.Cache == nil {
		return nil
	}
	v := view{name: "frozen", cutoff: t.FreezeAt, frozen: true}
	rows, err := l.compute(ctx, t, v.cutoff, "")
	if err != nil {
		return err
	}
	return l.write(ctx, t, v, rows)
}

func (l *Leaderboard) compute(ctx context.Context, t Times, cutoff time.Time, userID string) ([]Row, error) {
	params := db.ContestStandingsParams{
		ContestID: int64(t.ID),
		Cutoff:    pgtype.Timestamptz{Time: cutoff, Valid: true},
	}
	if userID != "" {
		params.UserID = pgtype.Text{String: userID, Valid: true}
	}
	standings, err := l.Queries.ContestStandings(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("contest standings: %w", err)
	}
	return Build(standings, t.StartsAt), nil
}

func (l *Leaderboard) read(ctx context.Context, id int, v string) ([]Row, bool, error) {
	set, hash := keys(id, v)
	results := l.Cache.DoMulti(ctx,
		l.Cache.B().Zrange().Key(set).Min("+inf").Max("-inf").Byscore().Rev().Build(),
		l.Cache.B().Hgetall().Key(hash).Build(),
	)
	members, err := results[0].AsStrSlice()
	if err != nil {
		return nil, false, err
	}
	encoded, err := results[1].AsStrMap()
	if err != nil {
		return nil, false, err
	}
	if len(members) == 0 {
		return nil, false, nil
	}
	rows := make([]Row, 0, len(members))
	for _, m := range members {
		raw, ok := encoded[m]
		if !ok {
			return nil, false, nil // half-written: rebuild
		}
		var r Row
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			return nil, false, err
		}
		rows = append(rows, r)
	}
	Rank(rows)
	return rows, true, nil
}

func (l *Leaderboard) expiry(t Times, v view) valkey.Completed {
	set, _ := keys(t.ID, v.name)
	if v.frozen {
		// The frozen view is needed until the end; keep it a week for the record.
		return l.Cache.B().Expireat().Key(set).Timestamp(t.EndsAt.Add(7 * 24 * time.Hour).Unix()).Build()
	}
	return l.Cache.B().Expire().Key(set).Seconds(int64(liveTTL / time.Second)).Build()
}

func (l *Leaderboard) hashExpiry(t Times, v view) valkey.Completed {
	_, hash := keys(t.ID, v.name)
	if v.frozen {
		return l.Cache.B().Expireat().Key(hash).Timestamp(t.EndsAt.Add(7 * 24 * time.Hour).Unix()).Build()
	}
	return l.Cache.B().Expire().Key(hash).Seconds(int64(liveTTL / time.Second)).Build()
}

func (l *Leaderboard) write(ctx context.Context, t Times, v view, rows []Row) error {
	set, hash := keys(t.ID, v.name)
	cmds := valkey.Commands{
		l.Cache.B().Multi().Build(),
		l.Cache.B().Del().Key(set, hash).Build(),
	}
	for _, r := range rows {
		cmd, err := l.rowCommands(set, hash, r)
		if err != nil {
			return err
		}
		cmds = append(cmds, cmd...)
	}
	cmds = append(cmds, l.expiry(t, v), l.hashExpiry(t, v), l.Cache.B().Exec().Build())
	return firstErr(l.Cache.DoMulti(ctx, cmds...))
}

func (l *Leaderboard) upsert(ctx context.Context, t Times, v view, r Row) error {
	set, hash := keys(t.ID, v.name)
	exists, err := l.Cache.Do(ctx, l.Cache.B().Exists().Key(set).Build()).AsInt64()
	if err != nil || exists == 0 {
		return err
	}
	cmds := valkey.Commands{l.Cache.B().Multi().Build()}
	rowCmds, err := l.rowCommands(set, hash, r)
	if err != nil {
		return err
	}
	cmds = append(cmds, rowCmds...)
	cmds = append(cmds, l.expiry(t, v), l.hashExpiry(t, v), l.Cache.B().Exec().Build())
	return firstErr(l.Cache.DoMulti(ctx, cmds...))
}

func (l *Leaderboard) rowCommands(set, hash string, r Row) (valkey.Commands, error) {
	encoded, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return valkey.Commands{
		l.Cache.B().Zadd().Key(set).ScoreMember().ScoreMember(r.Score(), r.UserID).Build(),
		l.Cache.B().Hset().Key(hash).FieldValue().FieldValue(r.UserID, string(encoded)).Build(),
	}, nil
}

func firstErr(results []valkey.ValkeyResult) error {
	var errs []error
	for _, r := range results {
		if err := r.Error(); err != nil && !valkey.IsValkeyNil(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// NewCache connects to Valkey (VALKEY_URL: redis://host:6379 locally, rediss:// with
// TLS in AWS). The caller runs without a cache when this fails: the leaderboard then
// comes from Postgres.
func NewCache(url string) (valkey.Client, error) {
	opt, err := valkey.ParseURL(url)
	if err != nil {
		return nil, err
	}
	// No client-side caching: boards change with every judged submission.
	opt.DisableCache = true
	return valkey.NewClient(opt)
}
