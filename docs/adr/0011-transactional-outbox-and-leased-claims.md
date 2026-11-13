# ADR-0011: Transactional outbox and lease-based claims

- Status: accepted
- Date: 2026-11-13

## Context

A new submission must reach the judge exactly when it is stored. Writing to the
database and then sending to SQS are two systems: if the process dies between them,
either the submission waits forever or the judge gets a message for a submission that
was rolled back. SQS standard queues deliver at least once and out of order, and a
worker can crash in the middle of judging.

## Decision

**Outbox.** `POST /api/submissions` inserts the submission and an `outbox_messages` row
in one transaction. A relay in every API task publishes pending rows to SQS:
`SELECT ... FOR UPDATE SKIP LOCKED` (sqlc), publish, mark sent, commit. A failed
publish backs off exponentially (`available_at`). Delivery is at-least-once.

**Leased claim.** The worker claims a submission with one conditional update:
`status = 'queued'`, or `status = 'running'` with `updated_at` older than the lease.
The lease equals the queue visibility timeout and exceeds the runner timeout.

Handling a message:

| Situation | Result | Message |
|---|---|---|
| Claim succeeds, runner OK | results + verdict in one transaction | delete |
| Claim fails, submission done/failed | duplicate delivery | delete |
| Claim fails, another worker holds the lease | wait | keep (redelivered) |
| Runner error, attempts left | status back to `queued` | keep (redelivered after the lease) |
| Runner error, no attempts left | verdict `IE` | delete |
| Malformed message or unknown submission | log | delete |

After `maxReceiveCount` deliveries SQS moves a message to the dead-letter queue.

## Consequences

- No lost and no phantom submissions; duplicates are harmless.
- A crashed worker delays a submission by at most one lease.
- The relay holds a short transaction while publishing; batches are small (10).
- Locally, ElasticMQ replaces SQS and the Lambda emulator replaces the runner function.
  The emulator serves one invocation at a time, so the local worker runs with concurrency 1.
- Covered by integration tests (relay with concurrent relays, all rows of the table
  above) and by the compose end-to-end test with the golden solutions.
