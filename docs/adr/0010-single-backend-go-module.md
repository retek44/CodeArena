# ADR-0010: One Go module for the backend

- Status: accepted (supersedes the Go part of ADR-0002)
- Date: 2026-11-13

## Context

Phase 1 had two Go modules joined by `go.work`: `api` and `worker`. In phase 3 the
worker needs the same database layer as the API (Ent client, sqlc queries, store),
and the API needs nothing from the worker's internals. Go's `internal` rule does not
allow one module to import another module's `internal` packages, and publishing them
as public packages with `replace` directives would add version friction without benefit.

The `go.work` setup also blocked `go run -mod=mod` (used by code generators) and
needed `GOWORK=off` workarounds.

## Decision

One module, `github.com/retek44/CodeArena/backend`, with several commands:

- `cmd/api`: HTTP API
- `cmd/worker`: queue consumer that judges submissions (phase 3)
- `cmd/judge`: local CLI for golden tests
- `cmd/migratediff`: migration generator

Shared packages live in `backend/internal/` (httpapi, auth, store, judge, checker,
runner, protocol, problem). Each command builds into its own container image.

## Consequences

- One `go.mod`, one dependency set, one lint and test run; no `go.work`.
- The API image does not include worker code, because Go links only the packages a
  command imports.
- A dependency bump affects both services at once; the CI runs all tests on every change.
