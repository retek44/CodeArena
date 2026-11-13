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

- [x] `contracts/openapi.yaml` (OpenAPI 3.1): problems, submissions with `Idempotency-Key`, RFC 9457 errors,
      cursor pagination
- [x] CI job "OpenAPI contract": Spectral with custom rules (problem+json errors, closed objects, summaries),
      `oasdiff` breaking-change check against the base branch (spec upload disabled)
- [x] Ent schema (users, problems, tags, test_cases, submissions, submission_results), ADR-0008
- [x] sqlc hot-path queries (claim, complete, requeue, fail, bulk results, stuck list)
- [x] Migration diff tool (Ent `NamedDiff` + Atlas dir format), dev database in docker compose, Makefile targets
- [x] Code generation run: Ent client, `20261111183343_initial.sql` + `atlas.sum`, sqlc package `internal/store/db`
      (Ent generator as a Go tool dependency; `golang.org/x/tools` v0.52 needed for Go 1.27)
- [x] `internal/store`: one pgx pool shared by Ent (`stdlib.OpenDBFromPool`) and sqlc
- [x] Integration tests with Testcontainers (`-tags integration`): concurrent claim wins once, complete only
      from running, idempotent result insert, idempotency key unique per user, stuck list + requeue
- [x] CI job "Database": generated-code drift check, `atlas migrate validate`, integration tests
- [ ] Migration lint (destructive-change check): `atlas migrate lint` or an open-source alternative
- [x] Problem and submission endpoints (`internal/httpapi`): cursor pagination (keyset for submissions),
      `Idempotency-Key` with request hash (replay 202, different body 409), private submissions (404 for others),
      RFC 9457 errors, hidden test cases never returned
- [x] Auth interface + `DevAuthenticator` (`AUTH_MODE=dev`, local only); no default mode (fails closed)
- [x] API integration tests with Testcontainers (shared `storetest` helper, template database per test)
- [x] N+1 query guard: pgx query tracer counts statements per request (problems page ≤ 2, submissions page ≤ 3)
- [x] Cognito user pool module (Essentials tier, email sign-in, optional TOTP MFA, SPA client with PKCE,
      managed login), wired into dev; ADR-0009
- [x] JWT verification (`internal/auth`, `AUTH_MODE=cognito`): RS256 only, JWKS cache, iss/exp/iat,
      `token_use=access`, `client_id`; 15 rejection cases tested (alg none, HS256 confusion, unknown kid, ...)
- [ ] Apply dev stack with Cognito in AWS and sign in end to end (with the frontend, phase 4)
- [x] Rate limit for submissions: sliding window per user counted in Postgres (default 10/min),
      429 + `Retry-After`, idempotent retries not counted
- [x] Contract test: every API integration test response is validated against `contracts/openapi.yaml`
      (libopenapi-validator, OpenAPI 3.1); a meta-test proves violations are caught
- [ ] SonarCloud quality gate

## Phase 3 – Async judge

- [x] One `backend` Go module (`cmd/api`, `cmd/judge`, `cmd/migratediff`; worker next), no `go.work` (ADR-0010)

- [x] Transactional outbox (`outbox_messages`, relay with `FOR UPDATE SKIP LOCKED`, exponential backoff); ADR-0011
- [x] Lease-based claim: duplicate deliveries are no-ops, a crashed worker's submission is taken over
- [x] `cmd/worker`: SQS consumer (long polling, bounded concurrency, graceful shutdown), retry budget then IE
- [x] Lambda invoker (AWS SDK) and HTTP invoker (runner container) behind one interface
- [x] `infra/modules/sqs`: queue + DLQ (redrive after 5 receives, SSE, long polling), wired into dev
- [x] Local stack in docker compose: migrations, seed, ElasticMQ, runner, API, worker
- [x] Tests: outbox relay (incl. 4 concurrent relays), worker outcomes, SQS consumer against ElasticMQ,
      E2E with all golden solutions through the whole chain (CI job "End-to-end")
- [ ] Worker on ECS Fargate (task definition, IAM, autoscaling on queue backlog) – with the phase 5 deploy
- [ ] Stuck-submission sweeper (`ListStuckSubmissions` → requeue + outbox) and DLQ alarm (phase 7)
- [ ] Checkers: float tolerance, custom
- [ ] More golden problems and languages in the verdict matrix
- [ ] Queue-backlog autoscaling, scale to zero
- [ ] C++ runner
- [ ] ECR push from CI, image scan, SBOM, cosign

## Phase 4 – Frontend + realtime

- [x] React 19 + Vite + TypeScript SPA (`frontend/`): problems, problem page with Monaco (lazy-loaded,
      bundled, no CDN), submissions with polling, my submissions, dev and Cognito (PKCE) sign-in
- [x] Typed API client generated from `contracts/openapi.yaml` (openapi-typescript + openapi-fetch);
      CI fails when the generated types drift from the contract
- [x] Vitest + Testing Library component tests (19), fetch stubbed per route
- [x] Playwright E2E against the compose stack with Page Object Model: AC, WA, CE, my submissions;
      axe WCAG 2.1 AA scan on three pages (no serious or critical violations); traces/videos on failure
- [x] CI: frontend job (types drift, ESLint, tsc, Vitest, build); Playwright in the E2E job; Semgrep TS/React rules;
      PR workflow also runs on pushes to main
- [ ] Live updates over WebSocket API instead of polling (push Lambda, `ws_connections`)
- [ ] Pact consumer tests (the OpenAPI-generated client and response validation cover most of it today)
- [ ] Firefox and WebKit projects in the main pipeline (`PW_ALL_BROWSERS=1`)

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
