# ADR-0016: Contests with clock-based phases, a Valkey leaderboard and a Step Functions lifecycle

- Status: accepted
- Date: 2026-11-13

## Context

PLAN.md Phase 6 asks for ICPC-style contests: a schedule, a live leaderboard, a freeze
before the end, final results and result e-mails. The lifecycle should run on AWS
services (Step Functions, EventBridge Scheduler), the leaderboard should not recompute
the standings from Postgres on every page view, and nothing in the lifecycle may decide
who is allowed to submit if it runs late or fails.

## Decision

- **Phases come from the clock.** `contest.PhaseAt(times, now)` derives scheduled,
  running, frozen and ended from `starts_at`, `freeze_at` and `ends_at`. The API and the
  worker use it to accept submissions and to choose the leaderboard view. The stored
  `status` column only records what the lifecycle has done (for example `final` after
  the results e-mails); a late or failed step changes nothing for the participants.
- **ICPC scoring:** solved problems first, then penalty = solve minute + 20 minutes per
  rejected attempt before the first AC. Compile errors and internal errors are not
  penalized. Equal scores share a rank.
- **Freeze:** the public board during the freeze is computed only from submissions created
  before `freeze_at`; a participant still sees their own verdicts on the submission page.
  After the end the full board is shown.
- **Postgres is the source of truth** (`ContestStandings` query, sqlc). Valkey
  (ElastiCache Serverless) caches each view as a sorted set `lb:{id}:<view>` plus a hash
  `lb:{id}:<view>:rows`; the `{id}` hash tag keeps both keys in one slot so they update
  in one `MULTI`. The worker updates the live view after each judged contest submission;
  a cache miss rebuilds the view from Postgres. Any Valkey error falls back to Postgres
  (`X-Leaderboard-Source: database`), so the cache can be lost at any time.
- **Lifecycle:** the seed creates one one-time EventBridge Scheduler schedule per new
  contest (group `codearena-<env>-contests`, deleted after it fires). It starts the state
  machine `codearena-<env>-contest`, which runs the `contest` ECS task
  (`ecs:runTask.sync`) for `start`, waits until `freeze_at` for `freeze`, until `ends_at`
  for `end`, then 120 seconds for the last verdicts and runs `finalize`. Each step
  publishes a `Contest phase changed` event on the judge event bus; `finalize` stores
  the final board and sends the result e-mails.
- **E-mails through SES**, optional: without `CONTEST_EMAIL_FROM` (a verified SES
  identity) the task sends none. Addresses are read from Cognito by user ID at finalize
  time; the database stores no e-mail addresses.
- **Live board:** leaderboard and phase events go over the WebSocket of ADR-0015 to
  subscribers of `contest#<slug>`; the SPA refetches the board over HTTP.
- The backend module derives the state machine, schedule group and scheduler role ARNs
  from their names, so the backend (which defines the task) does not depend on the
  contests module (which needs the task).

## Deviations from PLAN.md

- Schedules start at `max(starts_at, now + 2 minutes)`: Scheduler rejects one-time
  schedules in the past, and the clock-based phases make a late start harmless.
- No FIS experiment for a Valkey outage: the fallback to Postgres is covered by an
  integration test that stops the Valkey container.
- Contests are defined in `tests/golden/contests.json` and created by the seed; there is
  no admin UI.

## Consequences

- ElastiCache Serverless has a minimum charge per hour while it exists (bounded by the
  usage limits); the cache is created only in deployed environments and is destroyed with
  them.
- Step Functions standard workflows, Scheduler and the short ECS tasks cost cents per
  contest.
- Locally, docker compose runs a Valkey container; the lifecycle steps can be run by hand
  with `go run ./cmd/contest <step> <contest-id>`.
