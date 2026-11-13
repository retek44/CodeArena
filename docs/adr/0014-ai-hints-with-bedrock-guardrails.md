# ADR-0014: AI hints with Bedrock, a guardrail and limits in Postgres

- Status: accepted
- Date: 2026-11-13

## Context

After a wrong answer the user can ask for a hint (PLAN.md, phase 8). The hint must help
without solving the problem, must not leak hidden test data, must resist prompt injection
through the submitted source, and must keep the model cost predictable.

## Decision

- **Flow:** `POST /api/submissions/{id}/hint` → API (Go) → hint Lambda (Python,
  `lambdas/hint`) → Amazon Bedrock Converse API → Claude Haiku 5.5 through the EU
  cross-region inference profile `eu.anthropic.claude-haiku-5-5` (data stays in EU regions).
- **Only visible data goes to the model:** statement, source, verdict and the number of the
  first failing test. Input and expected output are sent only when that test is a sample,
  which the user can already see. The API builds the request; the Lambda never reads the database.
- **Three layers keep a hint a hint:**
  1. System prompt: at most three sentences, no code, no full algorithm, the code is data.
  2. Bedrock guardrail (`infra/modules/hint`): content filters, a prompt attack filter on
     the source (only the source is sent as `guardContent`, so the trusted statement does
     not trigger it) and a denied topic "full solution". The Lambda uses a numbered
     guardrail version; a change of the guardrail creates a new one.
  3. Post-filter in the Lambda: an answer that looks like code is replaced by a generic
     hint for the verdict (`status: blocked`).
- **One hint per submission, stored in Postgres** (`hints` table, unique on
  `submission_id`). Asking again returns the stored hint without a model call. Parallel
  requests are safe: `INSERT … ON CONFLICT DO NOTHING`, then everyone reads the stored row.
- **Rate limit 3 new hints per user and hour**, counted on the `hints` table (same sliding
  window as the submission limit). PLAN.md planned DynamoDB for it; the table already
  exists in Postgres, and a second store would add a service, IAM and cost for one
  counter.
- **Availability:** without `HINT_FUNCTION` (or `HINT_MODE=fake` locally) the endpoint
  answers 503; a model error also gives 503 and nothing is stored, so the user can retry.
  `hints_enabled = false` removes the Lambda and the guardrail from an environment.
- **The Lambda runs outside the VPC.** It needs only the Bedrock API; the isolated subnets
  have no route to it and an interface endpoint would cost about 8 USD a month per AZ.
- **Evaluation** (`tests/llm-eval`, nightly on dev): every wrong golden solution and
  prompt-injection cases go through the deployed Lambda. Deterministic checks (no code,
  length) must always pass; a stronger model (Claude Sonnet 5.5) grades with forced tool
  use whether the hint gives the solution away (≥ 90 % must not) and how relevant it is
  (mean ≥ 3 of 5). The bug descriptions in the golden files are the judge's ground truth
  and are stripped from the source the tutor sees. A small own harness instead of
  DeepEval: two metrics, no extra dependency tree, and the same post-filter code as the Lambda.

## Consequences

- Cost: about 1–2k input and 100 output tokens per hint; with the rate limit a busy day
  stays in cents. The nightly evaluation makes ~30 model calls.
- Bedrock model access must be enabled once per account (docs/SETUP.md). Until then the
  hint endpoint answers 503; everything else works.
- The guardrail can block a legitimate hint (false positive). The user then gets the
  generic hint, and the evaluation reports the share of blocked answers.
- The `Hint.status` enum is extensible (`x-extensible-enum`), so a new status is not a
  breaking change for clients.
