# CodeArena – Project Plan

CodeArena is a cloud-native online judge (a small LeetCode / Codeforces) on AWS.
Users solve programming problems in the browser, submit code, and see per-test-case
results in real time. Contests have a live leaderboard. An AI hint helps after a wrong answer.

The project has two goals:

1. **Development:** a non-trivial distributed system (sandboxed code execution, queue-driven
   workers, real-time updates, contests).
2. **Quality engineering:** a CI/CD pipeline with many test layers, quality gates and
   progressive delivery with automatic rollback.

Progress is tracked in [PROGRESS.md](PROGRESS.md). Decisions are in [adr/](adr/).

---

## 1. Product scope

- Problem list and problem page (Markdown statement, samples, limits).
- Browser editor (Monaco). Languages: Python and C++ first, then Java, Node.js, Go.
- Verdicts: `AC` accepted, `WA` wrong answer, `TLE` time limit, `MLE` memory limit,
  `OLE` output limit, `RE` runtime error, `CE` compile error, `IE` internal error
  (runner failed or ran out of time; the submission is retried).
- The runner reports how the program ran (`ok`, `time_limit`, ...); the worker decides the
  verdict by comparing outputs. Expected outputs never reach the runner.
- Live status per test case over WebSocket.
- Contests: scheduled start and end, live leaderboard, ICPC-style freeze in the last 30 minutes.
- AI hint on `WA` (Amazon Bedrock, Claude Haiku) with guardrails: no full solutions, rate limited.
- Admin: create problems, upload test cases, version problems.
- Problems are written for this project. Do not copy LeetCode or other judges (copyright).

## 2. Architecture

```
                    ┌──────────── CloudFront (+ WAF optional) ──────────────┐
Browser ──────────▶ │  /*  → S3 (React SPA)        /api/* → ALB            │
   │                └───────────────────────────────────┬──────────────────┘
   │ WebSocket                                          ▼
   ▼                                          ECS Fargate: api (Go)
API Gateway WebSocket ◀── push Lambda          │  Cognito JWT validation
   │ (connectionId → DynamoDB)                 │  RDS PostgreSQL
   │                                           │  ElastiCache Valkey
   │                                           ▼
   │                                    SQS submissions (+ DLQ)
   │                                           │
   │                                           ▼
   │                      ECS Fargate: judge-worker (Go), scales on queue backlog
   │                                 │  test cases from S3 (local cache)
   │                                 ▼
   │                  Runner Lambda per language (container image)
   │                  isolated subnet, no internet, Firecracker microVM
   │                                 │
   │                                 ▼
   └──────── EventBridge: SubmissionJudged ──▶ push Lambda / leaderboard / stats

Contest:  EventBridge Scheduler ──▶ Step Functions (start → freeze → end → ranking → SES email)
AI hint:  api ──▶ hint Lambda (Python) ──▶ Bedrock + Guardrails, rate limit in DynamoDB
Observability: OpenTelemetry (ADOT sidecar) ──▶ X-Ray + CloudWatch; alarms drive rollback
```

### Languages

- Go: API and judge worker. HTTP routing with the standard library (`net/http`, Go 1.22+ patterns).
- TypeScript: React frontend (Vite, TanStack Query, Monaco).
- Python: hint, WebSocket push and contest Lambdas; the Python runner.

## 3. Key decisions (details in `docs/adr/`)

- **Sandbox on Lambda** ([ADR-0003](adr/0003-sandbox-on-lambda.md)): Firecracker isolation,
  no network, `prlimit` limits, cleanup against execution-environment reuse.
- **At-least-once judge:** SQS can deliver twice. The worker claims a submission with a
  conditional update (`QUEUED → RUNNING`), so the judge is idempotent. After 3 failures the
  message goes to the DLQ and an alarm fires.
- **Queue-based autoscaling:** target tracking on backlog per task
  (`ApproximateNumberOfMessagesVisible / running tasks`). Scales to zero.
- **Network without NAT Gateway** ([ADR-0005](adr/0005-network-without-nat.md)).
- **Hybrid data access** ([ADR-0004](adr/0004-hybrid-data-access.md)): Ent for CRUD,
  sqlc for hot paths, Atlas for versioned migrations.
- **Expand/contract migrations:** every migration is backward compatible, so blue and green
  versions run on the same schema.
- **Preview environments** = shared dev base (VPC, RDS, Valkey) + per-PR app layer
  (own database `codearena_pr_<n>`, own ECS service, Lambdas and queues with `pr-<n>-` prefix).
- **Single AWS account** with environment prefixes ([ADR-0006](adr/0006-single-account-env-prefixes.md)).
- **CI authenticates with GitHub OIDC** ([ADR-0007](adr/0007-ci-auth-github-oidc.md)).

## 4. Data model

### PostgreSQL

- `users` (id = Cognito `sub`, username, role, created_at)
- `problems` (id, slug, title, difficulty, statement_md, time_limit_ms, memory_mb,
  checker_type [`exact` | `float_tolerance` | `custom`], version, published)
- `test_cases` (id, problem_id, problem_version, s3_key_in, s3_key_out, is_sample, weight)
- `submissions` (id UUID, user_id, problem_id, contest_id NULL, language, code,
  status [`QUEUED` | `RUNNING` | `DONE` | `FAILED`], verdict, runtime_ms, memory_kb, score,
  attempt, created_at, judged_at)
- `submission_results` (submission_id, test_case_id, verdict, runtime_ms, memory_kb)
- `contests` (id, title, starts_at, freeze_at, ends_at, status)
- `contest_problems`, `contest_participants`, `tags`, `problem_tags`

### DynamoDB

- `ws_connections` (PK `connectionId`, GSI `userId`, TTL)
- `rate_limits` (PK `userId#action#window`, TTL)

### Valkey

- `lb:{contestId}` sorted set (score and penalty time encoded in the sort score)
- `lb:{contestId}:frozen` snapshot at freeze time
- `problem:{id}` cache

### S3

- `testcases/{problemId}/v{n}/{i}.in|out`
- `submissions/{yyyy}/{mm}/{id}.src` (archive)
- frontend bucket

### Data access split

- **Ent:** problems, test cases, contests, users, admin endpoints (CRUD, relations, eager loading).
- **sqlc:** judge claim (`UPDATE ... WHERE status = 'QUEUED' RETURNING`), leaderboard
  fallback (window functions), bulk insert of `submission_results`.
- **Atlas:** `atlas migrate diff` generates versioned SQL from the Ent schema;
  `atlas migrate lint` blocks destructive changes in CI. No auto-migrate anywhere.

## 5. API (OpenAPI 3.1, spec first)

- `GET /api/problems`, `GET /api/problems/{slug}`
- `POST /api/submissions` → `202 Accepted` + id. `Idempotency-Key` header required.
- `GET /api/submissions/{id}`, `GET /api/users/me/submissions`
- `GET /api/contests`, `POST /api/contests/{id}/register`, `GET /api/contests/{id}/leaderboard`
- `POST /api/submissions/{id}/hint` (rate limit 3 per hour)
- `POST /api/admin/problems`, `POST /api/admin/problems/{id}/testcases` (presigned S3 URL)
- WebSocket: `subscribe {submissionId | contestId}` → `verdict_update`, `leaderboard_update`
- Event schemas: AsyncAPI + EventBridge Schema Registry

## 6. Repository layout (target)

```
api/            Go HTTP API (Ent + sqlc, Atlas migrations)
worker/         Go judge worker, verdict logic, checkers, local `judge` CLI
runners/        protocol/ (JSON Schemas), core/ (shared Python runner), one image dir per language
lambdas/        ws-push, hint, contest-steps (Python)
frontend/       React + Vite + TypeScript
contracts/      openapi.yaml, asyncapi.yaml
infra/          bootstrap/, shared/ (ECR), modules/, envs/{dev,staging,prod}/, preview/
tests/          golden/, sandbox-security/, api/, e2e/, contract/, performance/, chaos/, llm-eval/, infra/
docs/           PLAN.md, PROGRESS.md, SETUP.md, adr/
.github/        workflows/, actions/ (composite actions)
```

## 7. CI/CD (GitHub Actions, OIDC, actions pinned to commit SHA)

### Pull request (`pr.yml`)

Path filters run only the affected parts. Jobs run in parallel. `CI OK` is the single
required status check.

1. Static: golangci-lint, ruff, mypy, eslint, tsc, hadolint, actionlint, gitleaks, Semgrep.
   PR title must be a Conventional Commit (squash merge).
2. OpenAPI: spectral lint, breaking-change check (`oasdiff`) against main.
3. IaC: `terraform fmt/validate`, tflint, Checkov, `terraform plan` (job summary / PR comment),
   Infracost diff, Terratest for changed modules.
4. Unit: `go test -race -cover`, pytest, vitest; SonarCloud quality gate (≥ 80 % on new code).
5. DB: `atlas migrate lint`, generated-code drift check (`go generate` + `git diff --exit-code`),
   migration round-trip (`atlas schema diff`).
6. **Sandbox security suite:** runner image under the Lambda Runtime Interface Emulator.
   Fork bomb, infinite loop, memory bomb, network access, file system abuse, leftover
   background process. Each one must give the expected verdict and leave a clean environment.
7. **Golden judge tests:** matrix problem × language × expected verdict.
8. Contract: Pact (frontend ↔ API), event schema validation against AsyncAPI.
9. Integration: Testcontainers + AWS emulator. Full flow: submit → queue → worker → runner → verdict → event.
   N+1 query guard (query count per request).
10. Build: buildx arm64, image scan, SBOM (Syft), cosign keyless signing, push to ECR.
11. Preview environment: apply `infra/preview`, run migrations as a one-off ECS task, seed,
    smoke + Playwright `@smoke`, URL as PR comment. Destroyed when the PR closes.

### Main (`main.yml`)

1. Dev: deploy, migrate, full regression (Playwright sharded × 3 browsers, API tests,
   axe accessibility, visual regression, multi-user contest E2E).
2. Staging: `pact can-i-deploy`, deploy, then
   - k6: 200 concurrent submissions for 5 minutes; p95 judge latency < 10 s; no lost
     submissions; worker autoscaling triggered (checked through CloudWatch).
   - OWASP ZAP API scan from the OpenAPI spec.
   - AWS FIS: stop worker tasks mid-judge (all submissions still finish exactly once),
     runner latency injection (correct `TLE`), Valkey outage (leaderboard falls back to Postgres).
3. Manual approval (GitHub Environment `prod`).
4. Prod: expand migration; ECS blue/green with a lifecycle-hook smoke test; Lambda canary
   10 % for 5 minutes (CodeDeploy); CloudFront continuous deployment for the frontend;
   CloudWatch alarms trigger automatic rollback.
5. Post-deploy: CloudWatch Synthetics canary (login → submit → `AC`).
6. Release: semantic-release, changelog, GitHub Release with SBOM.

### Nightly (`nightly.yml`)

Full regression for all languages, mutation testing (Gremlins for Go, Stryker for TS),
Terraform drift detection (opens an issue), LLM eval for hints (DeepEval, LLM-as-judge),
auto-destroy of dev and staging at 22:00.

### Reporting

Allure report with history on GitHub Pages, job summaries, flaky-test detection
(retry once, tag as flaky, publish metric), quality dashboard in Grafana Cloud
(pass rate, duration, flaky trend, coverage, mutation score).

## 8. Observability

OpenTelemetry Go SDK → ADOT collector sidecar → X-Ray traces across
API → SQS → worker → runner → EventBridge. JSON logs with `trace_id`.
Custom metrics: judge latency, verdict distribution, queue age, runner cold starts.
The same alarms drive rollback and SNS notifications.

## 9. Cost

- One running environment costs about 0.10–0.20 USD per hour (ALB, two small arm64 Fargate
  tasks, `db.t4g.micro`, `cache.t4g.micro`). Lambda, S3, CloudFront and SQS cost cents.
- Expected: 15–40 USD per month with daily work. Prod is built only for demos.
- Guardrails: budget alerts (bootstrap), nightly auto-destroy, `make down ENV=<env>` after work.

## 10. Phases

| Phase | Scope | Weeks |
|---|---|---|
| 0 | Setup: account, budget, OIDC, state bucket, repo, pre-commit, PR pipeline | 1 |
| 1 | Runner MVP: Python runner Lambda, limits, sandbox security suite with RIE, local CLI judge | 2 |
| 2 | API + DB: OpenAPI, Ent + sqlc + Atlas, Cognito, problem and submission endpoints | 2 |
| 3 | Async judge: SQS, Fargate worker, checkers, golden corpus, idempotency, DLQ, autoscaling, C++ runner | 2 |
| 4 | Frontend + realtime: React + Monaco, WebSocket, Playwright POM, Pact | 2 |
| 5 | CD: dev/staging/prod, preview environments, blue/green, canary, rollback, migration task | 2 |
| 6 | Contests: Step Functions, Scheduler, Valkey leaderboard, freeze, SES | 1–2 |
| 7 | Non-functional: k6, ZAP, FIS, Synthetics, OTel dashboards | 2 |
| 8 | AI + polish: Bedrock hint + Guardrails, LLM eval, README, diagrams, demo video | 1–2 |

MVP milestone at the end of phase 3: code submitted from the CLI runs in the cloud and gets a verdict.
