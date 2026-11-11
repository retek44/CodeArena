# ADR-0001: Record architecture decisions

- Status: accepted
- Date: 2026-10-12

## Context

The project makes many design decisions that an interviewer or a future contributor will
ask about. The reasons get lost if they live only in commit messages.

## Decision

Record each significant decision as a short ADR in `docs/adr/`, numbered in order, with
the sections Context, Decision, Consequences and (if useful) Alternatives.
A decision that changes an earlier one adds a new ADR and marks the old one `superseded`.

## Consequences

- A PR that changes architecture adds or updates an ADR (PR template checklist).
- ADRs stay short: one page at most.
