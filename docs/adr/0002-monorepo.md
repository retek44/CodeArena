# ADR-0002: Monorepo

- Status: accepted
- Date: 2026-10-12

## Context

CodeArena has a Go API, a Go worker, runner images, Python Lambdas, a React frontend,
contracts, Terraform and many test suites. One person maintains everything.

## Decision

Keep everything in one repository. CI uses path filters so a change runs only the jobs
it affects. Shared contracts (`contracts/openapi.yaml`, `contracts/asyncapi.yaml`) live
next to both producer and consumer, so a contract change and its implementation land in
one PR.

## Consequences

- Atomic cross-component changes and one place for CI configuration.
- Path filters must be kept up to date, otherwise a job is skipped when it should run.
  The `CI OK` job treats skipped jobs as passing, so wrong filters hide problems.
- Each Go component is its own module (`api/`, later `worker/`), joined with `go.work`
  when the second module arrives.

## Alternatives

- Polyrepo: more realistic for large teams, but cross-repo contract changes and versioning
  add overhead without benefit for a single maintainer.
