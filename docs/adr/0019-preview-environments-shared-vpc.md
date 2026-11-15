# ADR-0019: Preview environments in a shared preview VPC

- Status: accepted
- Date: 2026-11-14

## Context

PLAN.md asks for a preview environment per pull request: deployed from the PR, URL as a
PR comment, destroyed when the PR closes. The plan sketched a "shared dev base (VPC, RDS,
Valkey) + per-PR app layer" with one database per PR on the dev RDS instance.

Facts that changed the design:

- dev is destroyed every night (`destroy.yml`). A preview inside dev's VPC or on dev's
  RDS instance would break that destroy (network interfaces and security groups still in
  use) or be broken by it.
- An account has five VPCs per region by default. Default VPC + dev + staging + prod is
  four, so one VPC per preview allows a single preview.
- The app module already builds a complete environment from variables,
  with migrations, seed and post-deploy tests in `deploy-environment.yml`. A second,
  "thin" app layer would duplicate it and drift from it.

## Decision

- **Shared network:** `infra/preview/base` is a VPC with public and isolated subnets
  (`codearena-preview-*`, 10.90.0.0/16). Without a NAT gateway or interface endpoints
  (ADR-0005) it costs nothing, so it stays; the preview pipeline applies it before every
  preview (a no-op once it exists).
- **Per PR:** `infra/preview/pr` calls the same `infra/modules/app` as dev, staging and
  prod with `env = "pr-<n>"` and the new `network` input, which skips the module's own
  VPC. State key `preview/pr-<n>/terraform.tfstate` (the `preview` deploy role may only
  write `preview/*`). The preview has its own RDS, ALB, CloudFront, Cognito pool, queue,
  Valkey cache and runner Lambdas, so it is isolated from every other environment.
  Tracing is off, one worker at most.
- **Opt-in:** the `preview` label starts it (`preview.yml`). Every push to the labelled
  PR redeploys; closing the PR or removing the label runs `destroy-preview.yml`, which
  also deletes the state file. A nightly job in `destroy.yml` lists the preview state
  files and destroys those whose PR is closed or unlabelled.
- **Same pipeline:** `deploy-environment.yml` takes `environment: preview` and a
  `pr_number`; the steps (targeted apply, migrate, seed, full apply, SPA upload, smoke and
  Playwright tests with a temporary Cognito user) are the ones dev uses.
- **Forks:** no preview. A fork's workflow gets no OIDC token for AWS, and only people
  with write access can add labels.

## Consequences

- A preview costs about as much as dev while it exists (RDS, ALB, Valkey, Fargate, public
  IPv4) and takes about 20 minutes to create (RDS, CloudFront). The label keeps that cost
  to the PRs that need it; the sweep keeps forgotten ones from running for long.
- Several previews can run at once without touching the VPC quota.
- Previews share a VPC, so their security groups, not the network, separate them; every
  database security group admits only its own environment's tasks.
- `module.network` became `module.network[0]` in the app module; a `moved` block keeps the
  existing dev, staging and prod networks in place.
