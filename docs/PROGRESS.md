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
- [x] **(manual)** Create the GitHub repo `retek44/CodeArena` and push
- [x] **(manual)** GitHub repo variables `AWS_ACCOUNT_ID`, `AWS_REGION`, `TF_STATE_BUCKET`
- [x] **(manual)** GitHub Environments `shared`, `preview`, `dev`, `staging`, `prod` (prod: required reviewer)
- [x] **(manual)** Branch protection on `main`: PR required, squash merge only, required checks
      `CI OK` and `Conventional PR title`
- [ ] **(manual)** Enable Dependabot alerts, secret scanning and push protection
- [x] Go module paths renamed to `github.com/retek44/CodeArena/{api,worker}`
- [x] First PR is green, including `terraform plan`

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
- [x] First deploy to dev (deploy.yml): images in ECR, runner Lambda, API and worker on ECS, smoke and
      post-deploy tests green; destroyed again with destroy.yml

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
- [x] Migration lint: `atlas migrate lint` (Community Edition image; the official build needs Atlas Pro)
      on the migrations a PR adds; destructive, data-dependent and incompatible changes fail
      (`backend/lint.atlas.hcl`, `make migrate-lint`)
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
- [x] SonarCloud quality gate (job `sonarcloud`: Go, TypeScript and Python coverage, `sonar.qualitygate.wait`;
      active once the secret `SONAR_TOKEN` exists); `npm audit` gate on runtime dependencies (dompurify
      pinned to 3.4.17 through `overrides`)

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
- [x] Worker on ECS Fargate (task definition, IAM, autoscaling on queue backlog) – with the phase 5 deploy
- [x] Stuck-submission sweeper (`internal/sweeper`, runs in every API task): running submissions whose worker
      went silent and queued ones whose message is gone (DLQ) are requeued and announced again through the
      outbox in one statement; exhausted ones get IE; `FOR UPDATE SKIP LOCKED` for parallel sweepers.
      Integration tests incl. 4 concurrent sweepers. DLQ alarm → SNS topic `codearena-<env>-alerts`
      (optional e-mail via the GitHub variable `ALERT_EMAIL`)
- [x] Checkers: `float` (numbers within an absolute or relative tolerance) and `custom` (problem's
      `checker.py`, run in the sandbox as a second runner invocation with `{input, expected, output}`;
      the submission run never sees expected outputs); migration `add_checkers`; per-case output budget
      so responses stay under the Lambda limit
- [x] More golden problems: `circle-area` (float checker) and `pair-sum` (custom checker, several
      correct answers, quadratic TLE solution); 15 golden solutions in the judge and E2E suites
- [x] Queue-backlog autoscaling, scale to zero
- [x] C++ runner (`runners/cpp`, GCC, static linking; ADR-0013): one Lambda function per language,
      worker routes by language, language picker in the SPA, 8 C++ golden solutions, Playwright C++ test
- [x] ECR push from CI, image scan (Grype gate: no fixable critical CVE, in the PR jobs and before
      signing), SBOM, cosign

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
- [x] Live updates over WebSocket (ADR-0015): worker -> EventBridge -> push Lambda -> API Gateway WebSocket;
      single-use tickets (`POST /api/ws/ticket`, DynamoDB, consumed by `$connect`), ownership checked at
      push time, `contracts/asyncapi.yaml` validated by the Go and Python tests; the SPA refetches on an
      update, polls every 15 s while connected and every second without a socket
- [x] Pact contract (ADR-0017): SPA consumer tests (`npm run test:pact`, 12 interactions through the real
      request functions) write `frontend/pacts`; the Go provider test verifies them against the real
      handler and database (Testcontainers, fresh database per provider state); CI job `contract`
- [x] Firefox and WebKit projects in the main pipeline (`PW_ALL_BROWSERS=1` on pushes to main; PRs: Chromium)

## Phase 5 – CD

- [x] `infra/modules/app`: one module per environment (dev, staging, prod differ only in variables);
      base without image tag, full stack with it
- [x] `infra/modules/postgres`: RDS PostgreSQL 18 (t4g.micro, isolated subnets, TLS forced, password
      managed by RDS in Secrets Manager, never in state)
- [x] `infra/modules/backend`: ECS Fargate arm64 (API service + worker on Fargate Spot), ALB reachable
      only from CloudFront, circuit breaker and 5xx-alarm rollback, worker scaling 0..N on queue depth,
      migration task, least-privilege task roles with the workload boundary
- [x] `infra/modules/frontend-cdn`: private S3 + CloudFront (OAC), `/api/*` to the ALB, SPA routing
      function, CSP/HSTS security headers; ADR-0012
- [x] Migration image (`backend/Dockerfile.migrate`, Atlas) and DB settings from parts (`DB_*`)
- [x] `deploy.yml` (manual): shared ECR apply → arm64 images with SBOM and keyless cosign signature,
      built once per commit → expand migration task → full apply → wait for rollout → SPA build with
      Cognito settings → S3 + invalidation → smoke test
- [x] `destroy.yml`: nightly destroy of dev and staging, manual destroy of any environment
- [x] PR workflow: Terraform plan for shared, dev, staging, prod (needs the repo variables)
- [x] **(manual)** GitHub repo variables and environments (`docs/SETUP.md` 4.), then the first dev deploy
- [x] Seed task: `backend/Dockerfile.seed` bakes `tests/golden` into the image (named build context),
      idempotent; deploy runs migrate then seed as one-off ECS jobs (`.github/scripts/run-ecs-task.sh`);
      docker compose uses the same image; problem statements from `statement.md`
- [x] `terraform test` with mocked providers for `infra/modules/app` (plan-only, no AWS): caught a
      `for_each` over unknown security group IDs that `validate` missed
- [ ] Preview environments per PR (`infra/preview`) + cleanup workflow
- [ ] ECS blue/green with test-listener hooks, Lambda canary (CodeDeploy)
- [x] Reusable workflows: `build-images.yml` (once per commit) and `deploy-environment.yml`;
      `deploy.yml` deploys one environment, `release.yml` promotes main dev → staging → prod
      (prod waits for the environment reviewer)
- [x] Post-deploy Playwright suite (`frontend/e2e/deployed`, `E2E_BASE_URL`): seeded problem list +
      axe, SPA deep link through the CloudFront function, security headers on SPA and API, no edge
      caching of `/api/*`, HTTP → HTTPS redirect, ALB unreachable around CloudFront
- [x] Signed-in post-deploy test (`e2e/deployed/signed-in.spec.ts`): the pipeline creates a temporary
      Cognito user, signs it in with `ADMIN_USER_PASSWORD_AUTH` (IAM only), runs Python, C++ and
      custom-checker submissions through the real queue, worker and runner Lambdas, then deletes the user
- [x] release-please (`release-please.yml`): release PR with version bump and CHANGELOG from the
      Conventional Commit squash titles; the tag publishes a GitHub Release with an SPDX SBOM

## Phase 6 – Contests

- [x] Contests (ADR-0016): `contest`, `contest_problem`, `contest_participant` tables, contest
      submissions (`POST /api/submissions` with `contest`), list/detail/register/leaderboard endpoints,
      contests pages in the SPA (register, problems A/B/C, ICPC board, freeze notice)
- [x] Phases from the clock (scheduled, running, frozen, ended); ICPC scoring (solve minute + 20 per
      rejected attempt, CE/IE free, shared ranks); frozen board from submissions before `freeze_at`
- [x] Valkey leaderboard cache (ElastiCache Serverless; sorted set + hash per view, worker updates the
      live view, Postgres fallback on any cache error, integration test stops Valkey)
- [x] Lifecycle: seed creates a one-time EventBridge Scheduler schedule per contest -> Step Functions
      (`ecs:runTask.sync` of the `contest` task) for start, freeze, end, finalize; phase and leaderboard
      events over the WebSocket (`contest#<slug>`)
- [x] SES result e-mails at finalize (optional `CONTEST_EMAIL_FROM`, addresses from Cognito)
- [ ] Verify on AWS: schedule fires, state machine runs all four steps, result e-mail arrives

## Phase 7 – Non-functional

- [x] k6 load test (`tests/performance/submissions.js`): signed-in users through the whole judge chain;
      thresholds on p95 judge latency (10 s), lost submissions (0) and wrong verdicts (0); warm-up first
- [x] Worker autoscaling verified after the load (Application Auto Scaling activities)
- [x] AWS FIS experiment template (`infra/modules/backend/chaos.tf`): all worker tasks stopped under load,
      every submission must still be judged (k6 thresholds)
- [x] OWASP ZAP API scan from the OpenAPI spec through CloudFront (warnings fail; rule overrides in
      `tests/security/zap-rules.tsv`); API sends `nosniff`, CORP and `no-store` itself
- [x] `nonfunctional.yml`: k6 + scaling check + FIS + ZAP on staging; `release.yml` gates prod on it
- [x] CloudWatch dashboard per environment (`infra/modules/observability`): ALB, queue + DLQ, ECS, runner
      Lambdas, RDS, and Logs Insights widgets for verdicts by language, judge latency and sweeper activity
      (no custom-metric cost)
- [x] Synthetics canary (prod): sign in -> submit -> AC every 30 minutes, fresh random password per run,
      failure alarm to the alerts topic
- [x] OpenTelemetry traces (ADR-0018): one trace per submission, API -> outbox (`trace_parent` column)
      -> SQS (message attributes) -> worker -> runner Lambda (X-Amzn-Trace-Id) -> EventBridge -> WebSocket
      Lambda; ADOT collector sidecar exports to X-Ray, sqlc-named DB spans, `trace_id` in JSON logs,
      submission ID as X-Ray annotation; locally `make local-traces` (Jaeger)
- [x] `nightly.yml`: mutation testing (Gremlins on the Go judge logic with an efficacy threshold, Stryker on
      the SPA with a break threshold; scores in the job summary) and Terraform drift detection (plans
      every running stack with its deployed image tag, opens or updates a `drift` issue)

## Phase 8 – AI + polish

- [x] AI hint (ADR-0014): `POST /api/submissions/{id}/hint` (own WA/RE/TLE/MLE/OLE submissions, one stored
      hint per submission, 3 new hints per user and hour, 503 without a hint service), only sample test
      data goes to the model
- [x] Hint Lambda (`lambdas/hint`, Python): Bedrock Converse with Claude Haiku 5.5 (EU inference profile),
      system prompt + code post-filter with a generic fallback; pytest with a stubbed client
- [x] `infra/modules/hint`: Bedrock guardrail (content + prompt attack filters, denied topic "full
      solution", numbered version), Lambda outside the VPC, least-privilege IAM; `hints_enabled` per env
- [x] SPA: "Get a hint" on failed submissions, rate-limit message with the wait time
- [x] LLM eval in `nightly.yml` (`tests/llm-eval`): wrong golden solutions + prompt-injection cases
      through the dev Lambda, deterministic checks + LLM-as-judge (Sonnet 5.5, forced tool use)
- [ ] **(manual)** Bedrock model access for Claude Haiku 5.5 and Sonnet 5.5 (`docs/SETUP.md` 1.5)
- [ ] README with diagrams, demo video, final ADRs
