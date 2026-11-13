# ADR-0015: Live updates over an API Gateway WebSocket with single-use tickets

- Status: accepted
- Date: 2026-11-13

## Context

The submission page polled every second until the verdict arrived (PLAN.md wants live
status over WebSocket). The API runs on ECS behind CloudFront and an ALB; WebSockets
through that path would pin connections to API tasks and complicate scaling and deploys.
API Gateway WebSocket APIs offer no JWT authorizer, and browsers cannot send an
`Authorization` header when they open a WebSocket.

## Decision

- **Path:** worker → EventBridge bus `codearena-<env>` (`Submission updated`, source
  `codearena.judge`) → rule → push Lambda → API Gateway WebSocket (`/live` stage) →
  browser. The contract is `contracts/asyncapi.yaml`; the Go publisher and the Python
  Lambdas test their messages against its schemas.
- **Thin messages:** the socket says only "this submission changed" (status, verdict).
  The SPA then reads the submission over HTTP as before, so there is one source of data
  and one authorization path for it.
- **Authentication with a single-use ticket:** `POST /api/ws/ticket` (normal Cognito auth)
  writes a random 256-bit ticket with the user ID into DynamoDB for 60 seconds. The
  `$connect` Lambda deletes it in the same call that reads it, so a ticket opens at most
  one connection; no ticket, no connection. The ticket is in the query string, which
  API Gateway logs do not record in our access log format.
- **Ownership at push time:** anyone may subscribe to any submission ID, but an update
  is sent only to connections of the user who owns the submission (the event carries
  the owner). Nothing to guess, nothing leaked.
- **Best effort, polling as safety net:** events are published after the result is
  committed and are not retried by the worker. While the socket is open the SPA polls
  every 15 s instead of every second; without a WebSocket API (local stack, ticket
  endpoint 503) or after a socket error it polls every second as before.
- **One DynamoDB table** (tickets, connections, subscriptions; TTL cleans up) instead of
  the `ws_connections` table plus a GSI from PLAN.md: the push queries subscriptions by
  submission, which is the partition key.
- Lambdas run outside the VPC (DynamoDB and the management API only).

## Consequences

- Cost is per use: WebSocket messages and minutes, Lambda invocations, DynamoDB
  on-demand requests, EventBridge events. An idle environment costs nothing.
- The CSP `connect-src` of the SPA includes the WebSocket origin of the environment.
- A lost event delays an update by at most the 15 s safety poll.
- Contests can publish their own events (leaderboard updates) on the same bus and socket.
