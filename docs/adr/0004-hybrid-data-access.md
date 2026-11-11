# ADR-0004: Hybrid data access (Ent + sqlc + Atlas)

- Status: accepted
- Date: 2026-10-12

## Context

Most of the API is CRUD with relations (problems, test cases, contests, users). A few
queries are correctness- or performance-critical:

- claiming a submission exactly once (`UPDATE ... WHERE status = 'QUEUED' RETURNING ...`),
- leaderboard fallback with window functions (`RANK() OVER (...)`),
- bulk insert of per-test-case results.

Blue/green deployments run two application versions on one database at the same time,
so schema changes must be backward compatible.

## Decision

- **Ent** for CRUD and relations: schema as Go code, generated type-safe client, eager
  loading (`WithTestCases()`).
- **sqlc** for the hot paths: plain SQL, checked at generate time, generated Go functions.
  Both use the same `database/sql` connection pool.
- **Atlas** for migrations: `atlas migrate diff` writes versioned SQL files from the Ent
  schema. They are reviewed in the PR and applied by a one-off ECS task in the pipeline.
- **Never auto-migrate** at application start.

CI gates:

- `atlas migrate lint` fails on destructive or backward-incompatible changes
  (drop column, add `NOT NULL` without default, ...), which enforces expand/contract.
- `go generate ./...` followed by `git diff --exit-code` fails when generated code is stale.
- Migration round trip: apply all migrations to an empty database and compare with the
  schema Ent expects (`atlas schema diff`).
- Integration tests run against real PostgreSQL (Testcontainers), never SQLite, and assert
  a maximum number of queries per request to catch N+1 problems.

## Consequences

- Fast CRUD development and explicit SQL where it matters.
- Two code generators to maintain. Clear rule: if a query needs `RETURNING`, window
  functions, `FOR UPDATE SKIP LOCKED` or bulk operations, it goes to sqlc.

## Alternatives

- sqlc only: maximum control, a lot of boilerplate for CRUD.
- GORM: well known, but reflection-based, weaker type safety and tempting `AutoMigrate`.
