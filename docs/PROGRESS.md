# Progress

Status of each phase in [PLAN.md](PLAN.md). Update this file in every PR that finishes an item.

Legend: `[x]` done · `[ ]` open · **(manual)** must be done by a human in AWS or GitHub.

## Phase 0 – Setup

### In the repository (done)

- [x] Repo layout, `.gitignore`, `.gitattributes`, `.editorconfig`
- [x] `infra/bootstrap`: state bucket (versioning, SSE-S3, TLS-only policy, lifecycle, public access block)
- [x] `infra/modules/github-oidc`: OIDC provider, read-only plan role, deploy role per GitHub Environment
      (`shared`, `preview`, `dev`, `staging`, `prod`), workload permissions boundary
- [x] `infra/modules/budget`: monthly budget with alerts at 20 / 50 / 100 % and forecast 100 %
- [x] `infra/envs/{dev,staging,prod}`: stacks with S3 backend (native locking) and default tags
- [x] Provider lock files for linux, windows and darwin
- [x] `api`: Go service skeleton (`/healthz`, `/version`, graceful shutdown, JSON logs), table-driven test,
      distroless arm64 Dockerfile
- [x] `.github/workflows/pr.yml` and `pr-title.yml`, actions pinned to commit SHA, Dependabot
- [x] `.pre-commit-config.yaml`, `Makefile`, `docker-compose.yml` (Postgres 18, Valkey 8, API)
- [x] ADRs 0001–0007

### Manual steps (see [SETUP.md](SETUP.md))

- [x] **(manual)** Install tools (Go 1.27, Terraform 1.17, AWS CLI 2.39)
- [x] **(manual)** Standalone AWS account `CodeArena` (379269296137), paid plan, root MFA, Free Tier alerts.
      (The first account, "Fresh Deploy" 207871833442, is a preconfigured project whose SCPs deny
      S3, Lambda, ECS, EC2 and more; it is not used.)
- [x] **(manual)** IAM admin user `admin_` + CLI profile `CodeArena`
- [x] **(manual)** `infra/bootstrap` applied (28 resources, budget 5 USD to the alert email),
      state migrated to `s3://codearena-tfstate-379269296137-eu-central-1/bootstrap/`
- [x] **(manual)** `infra/backend.hcl` created from the bootstrap output
- [ ] **(manual)** Create the GitHub repo `retek44/CodeArena` and push
- [ ] **(manual)** GitHub repo variables `AWS_ACCOUNT_ID`, `AWS_REGION`, `TF_STATE_BUCKET`
- [ ] **(manual)** GitHub Environments `shared`, `preview`, `dev`, `staging`, `prod` (prod: required reviewer)
- [ ] **(manual)** Branch protection on `main`: PR required, squash merge only, required checks
      `CI OK` and `Conventional PR title`
- [ ] **(manual)** Enable Dependabot alerts, secret scanning and push protection
- [x] Go module paths renamed to `github.com/retek44/CodeArena/{api,worker}`
- [ ] First PR is green, including `terraform plan`

### Deferred to later phases

- Container image vulnerability scan (Trivy or Grype) – phase 3 with ECR push
- SonarCloud, Infracost – need accounts and tokens; phase 2
- AWS emulator for integration tests (check the license and auth-token requirement first) – phase 3

## Phase 1 – Runner MVP

- [x] `runners/protocol`: JSON Schemas for request and response (protocol v1)
- [x] `runners/core`: runner package (request validation, limits, per-case fresh artifacts, sandbox reset,
      environment recycle when the reset fails), see ADR-0003
- [x] `runners/python`: Lambda container image on `public.ecr.aws/lambda/python:3.13`, non-root user
- [x] Runner unit and contract tests (pytest: parser ↔ schema agreement, status classification, limits)
- [x] `worker` Go module: protocol types, runner HTTP client, exact checker, verdict logic, problem loader
- [x] `judge` CLI: `make judge SRC=tests/golden/a-plus-b/solutions/ac.py`
- [x] Golden corpus seed: `tests/golden/a-plus-b` with AC, WA, TLE, MLE, RE, CE solutions; `TestGolden`
- [x] CI: Go matrix (`api`, `worker`), `runner-python` job (ruff, mypy, pytest, hadolint, arm64 image,
      container started with Lambda-like restrictions, golden tests)
- [x] `infra/modules/network`: VPC, public subnets, isolated subnets (no route out), default SG without rules
- [x] `infra/modules/ecr-repository` + `infra/shared` stack (ECR for api, worker, runner-python)
- [x] `infra/modules/runner-lambda`: arm64 image function in isolated subnets, SG without rules,
      boundary-limited role, X-Ray, log group; wired into `infra/envs/dev` (created once an image tag is given)
- [ ] Sandbox hardening test suite (`tests/sandbox-security`) – open
- [ ] First deploy to dev: push runner image to ECR, `terraform apply` with `runner_python_image_tag`,
      invoke from the CLI through the Lambda API (needs the AWS account)

## Phase 2 – API + DB

- [ ] `contracts/openapi.yaml`, spectral lint, `oasdiff` breaking-change check
- [ ] Ent schema, Atlas migrations, `atlas migrate lint` in CI
- [ ] sqlc queries for hot paths
- [ ] Cognito user pool, JWT middleware
- [ ] Problem and submission endpoints, `Idempotency-Key`
- [ ] Testcontainers integration tests, N+1 query guard
- [ ] SonarCloud quality gate

## Phase 3 – Async judge

- [ ] SQS queue + DLQ, worker on Fargate, claim with conditional update
- [ ] Lambda invoker in the worker (AWS SDK), replacing the HTTP invoker in AWS
- [ ] Checkers: float tolerance, custom
- [ ] More golden problems and languages in the verdict matrix
- [ ] Queue-backlog autoscaling, scale to zero
- [ ] C++ runner
- [ ] ECR push from CI, image scan, SBOM, cosign

## Phase 4 – Frontend + realtime

- [ ] React + Vite + Monaco, Cognito login
- [ ] WebSocket API, push Lambda, `ws_connections` table
- [ ] Playwright with Page Object Model, Pact consumer tests

## Phase 5 – CD

- [ ] `main.yml`: shared → dev → staging → approval → prod
- [ ] Network and runner modules wired into staging and prod
- [ ] Preview environments per PR (`infra/preview`) + cleanup workflow
- [ ] Migrations as a one-off ECS task
- [ ] ECS blue/green with lifecycle hook tests, Lambda canary, CloudWatch alarm rollback
- [ ] semantic-release

## Phase 6 – Contests

- [ ] Step Functions lifecycle, EventBridge Scheduler, Valkey leaderboard, freeze, SES

## Phase 7 – Non-functional

- [ ] k6 load tests with thresholds, ZAP, FIS experiments, Synthetics canary, dashboards

## Phase 8 – AI + polish

- [ ] Bedrock hint Lambda + Guardrails + rate limit, LLM eval in nightly
- [ ] README with diagrams, demo video, final ADRs
