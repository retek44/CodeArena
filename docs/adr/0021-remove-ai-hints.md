# ADR-0021: Remove the AI hints

- Status: accepted (supersedes [ADR-0014](0014-ai-hints-with-bedrock-guardrails.md))
- Date: 2026-11-15

## Context

ADR-0014 added AI hints: after a failed submission the API called a Lambda that asked a
Claude model on Amazon Bedrock, behind a guardrail, for a hint. Every hint was a paid model
call on the project's AWS account. The limits (one hint per submission, three per user and
hour, failed submissions only) were per user, and Cognito allows self sign-up, so a script
with many accounts could still run up the bill. The nightly LLM evaluation called the
models too.

The project is a demo that anyone may open. Its owner does not want any feature that turns
public traffic into token cost.

## Decision

Remove the feature completely instead of capping it:

- API: `POST /api/submissions/{id}/hint` is gone (the oasdiff check approves the removal in
  `contracts/breaking-changes-approved.txt`), with the hint service, the rate limit and the
  Ent schema.
- SPA: no "Get a hint" panel; the Pact contract has no hint interaction.
- AWS: no hint Lambda, no guardrail, no `hints_enabled` or model variables; the nightly LLM
  evaluation (`tests/llm-eval`) is deleted.
- IAM: `bedrock` is no longer in the workload permissions boundary, so no role the pipeline
  creates can call a model even by mistake, and the CI deploy roles deny `bedrock:*`
  explicitly (PowerUserAccess would allow it). Both take effect when the bootstrap stack is
  applied again.
- There never was an API key: the Lambda used its IAM role. Nothing in the repository or
  its history grants model access.

## Consequences

- No code path in the project calls a generative AI model, and the account's CI roles cannot.
- The `hints` table stays in the database for now: dropping it in the same release would
  break the API version that blue/green keeps running during the switch (expand/contract).
  A later migration drops it (`-- atlas:nolint destructive`, it holds no data anyone needs).
- Bedrock model access in the AWS console is no longer needed; if it was granted, it can
  stay or be removed, it costs nothing without calls.
- The remaining public cost surface is the judge itself (runner Lambda invocations, a few
  per submission, rate limited to 10 per user and minute). Its unit cost is far lower than
  a model call; a global cap is a possible next step if the demo stays public.
