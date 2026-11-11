# ADR-0006: Single AWS account with environment prefixes

- Status: accepted
- Date: 2026-10-12

## Context

The AWS best practice is one account per environment (AWS Organizations). Free plan
credits belong to one account, and a multi-account setup adds work that does not change
how the application works.

## Decision

Use one account. Separate environments by:

- resource name prefix `codearena-<env>-` (preview: `codearena-pr-<n>-`),
- the `Environment` default tag,
- separate Terraform state keys (`envs/<env>/`, `preview/`),
- separate CI deploy roles, each assumable only from its GitHub Environment.

## Consequences

- Cheap and simple.
- The blast radius is the whole account: deploy roles are limited by name prefix, not by
  environment. A later step can add tag-based conditions (ABAC) or move to AWS Organizations
  with one account per environment; the Terraform layout (`envs/<env>`) already supports that.
