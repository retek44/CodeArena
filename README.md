# CodeArena

A cloud-native online judge on AWS. Solve programming problems in the browser, submit
Python or C++ code, watch the verdict of every test case arrive live, and compete in
contests with an ICPC-style leaderboard.

The project has two goals: a non-trivial distributed system (sandboxed code execution,
queue-driven workers, real-time updates, scheduled contests), and a delivery pipeline with
many test layers, quality gates and progressive delivery with automatic rollback.

> **Status:** feature-complete for the plan in [docs/PLAN.md](docs/PLAN.md); open items and
> AWS verifications are tracked in [docs/PROGRESS.md](docs/PROGRESS.md).

## Demo

![Demo: sign in, a wrong answer with per-test-case results, the fix accepted, a contest leaderboard](docs/media/demo.gif)

Recorded with Playwright against the local stack (`make demo-video`).

## Highlights

- **Sandboxed execution:** untrusted code runs in runner Lambdas (Firecracker microVMs) in
  subnets without any route out, with CPU, memory, process, file and output limits; the
  runner never sees expected outputs ([ADR-0003](docs/adr/0003-sandbox-on-lambda.md)).
- **Queue-driven judge:** transactional outbox, SQS with a DLQ, Go workers on Fargate Spot
  with leased, idempotent claims, a sweeper for stuck submissions, scaling to zero on queue
  backlog ([ADR-0011](docs/adr/0011-transactional-outbox-and-leased-claims.md)).
- **Real time:** verdicts and leaderboards pushed through EventBridge and API Gateway
  WebSockets with single-use tickets ([ADR-0015](docs/adr/0015-live-updates-websocket-tickets.md)).
- **Contests:** phases from the clock, EventBridge Scheduler and Step Functions for the
  lifecycle, Valkey leaderboard with freeze and a Postgres fallback, result e-mails through SES
  ([ADR-0016](docs/adr/0016-contests-clock-phases-valkey-step-functions.md)).
- **No paid AI calls:** the first plan had Bedrock hints; they were removed, and IAM denies
  Bedrock, so public traffic cannot turn into model cost ([ADR-0021](docs/adr/0021-remove-ai-hints.md)).
- **Delivery:** GitHub Actions with OIDC (no AWS keys), images built once and promoted
  dev → staging → prod, preview environment per pull request, ECS blue/green with a
  smoke-test hook and a CodeDeploy canary for the runners, alarm-based rollback
  ([ADR-0019](docs/adr/0019-preview-environments-shared-vpc.md), [ADR-0020](docs/adr/0020-blue-green-api-canary-runners.md)).
- **Observability:** one OpenTelemetry trace per submission across API, outbox, SQS, worker,
  runner and WebSocket push (ADOT to X-Ray), CloudWatch dashboard, Synthetics canary
  ([ADR-0018](docs/adr/0018-opentelemetry-traces-adot-xray.md)).

## Architecture

```mermaid
flowchart LR
  user([Browser])

  subgraph edge[Edge]
    cf[CloudFront]
    s3[(S3: React SPA)]
    cognito[Cognito<br/>managed login, PKCE]
  end

  subgraph vpc[VPC without NAT gateway]
    subgraph public[Public subnets]
      alb[ALB<br/>CloudFront only]
      api[API<br/>Go on Fargate]
      worker[Worker<br/>Go on Fargate Spot]
    end
    subgraph isolated[Isolated subnets: no route out]
      rds[(RDS PostgreSQL)]
      valkey[(Valkey<br/>leaderboard)]
      runners[Runner Lambdas<br/>Python, C++]
    end
  end

  sqs[[SQS submissions<br/>+ DLQ]]
  bus{{EventBridge}}
  push[WebSocket push Lambda]
  apigw[API Gateway<br/>WebSocket]
  xray[X-Ray]

  user -->|HTTPS| cf
  cf -->|/*| s3
  cf -->|/api/*, secret header| alb --> api
  user -. sign in .-> cognito
  api --> rds
  api --> valkey
  api -->|outbox relay| sqs --> worker
  worker -->|invoke live alias| runners
  worker --> rds
  worker --> valkey
  worker -->|Submission updated| bus --> push --> apigw -->|live verdicts| user
  api -. ADOT sidecar .-> xray
  worker -. ADOT sidecar .-> xray
```

Each environment (dev, staging, prod, `pr-<n>`) is the same Terraform module
(`infra/modules/app`) with different variables, so environments never differ in structure.

### One submission

```mermaid
sequenceDiagram
  autonumber
  participant B as Browser
  participant A as API
  participant DB as PostgreSQL
  participant Q as SQS
  participant W as Worker
  participant R as Runner Lambda
  participant E as EventBridge + WebSocket

  B->>A: POST /api/submissions (Idempotency-Key)
  A->>DB: insert submission + outbox row (one transaction)
  A-->>B: 202 Accepted
  A->>Q: outbox relay sends the message (trace context attached)
  Q->>W: receive (at least once)
  W->>DB: claim with a lease (QUEUED to RUNNING, duplicates are no-ops)
  W->>R: source + test inputs (never the expected outputs)
  R-->>W: status, time, memory and stdout per test case
  W->>W: verdict per case (exact, float or sandboxed custom checker)
  W->>DB: results + verdict
  W->>E: Submission updated
  E-->>B: push to the owner's connections only
```

A crashed worker's lease expires and SQS delivers again; submissions that stay stuck are
requeued by the sweeper, and exhausted ones end as `IE`.

### Contest lifecycle

```mermaid
flowchart LR
  seed[seed task] -->|one-time schedules| sch[EventBridge Scheduler]
  sch -->|start, freeze, end, finalize| sfn[Step Functions]
  sfn -->|ecs:runTask.sync| task[contest task]
  task --> db[(PostgreSQL)]
  task --> vk[(Valkey board)]
  task -->|phase changed| bus{{EventBridge}} --> ws[WebSocket]
  task -->|finalize| ses[SES result e-mails]
```

Phases come from the clock, so who may submit never depends on a schedule firing on time;
the lifecycle steps refresh caches, push events and send the results.

## Delivery pipeline

```mermaid
flowchart TB
  subgraph pr[Pull request: pr.yml]
    direction LR
    lint[actionlint, gitleaks,<br/>Semgrep, Spectral, oasdiff] --> unit[Go, TS and Python<br/>unit tests, Sonar]
    unit --> integ[Testcontainers integration,<br/>Pact, migration lint]
    integ --> e2e[docker compose E2E:<br/>golden solutions, Playwright, axe]
    e2e --> tf[Terraform validate, tftest,<br/>tflint, Checkov, plan]
  end

  label{{label preview}} -->|preview.yml| preview[pr-n environment,<br/>URL as PR comment,<br/>destroyed on close]

  subgraph release[Release: release.yml]
    direction LR
    build[build once: arm64 images,<br/>Grype, SBOM, cosign] --> dev[dev] --> staging[staging] --> nf[k6 load, FIS chaos,<br/>ZAP scan] --> approve{{reviewer}} --> prod[prod]
  end

  pr -->|squash merge| release
  nightly[nightly.yml: mutation testing,<br/>drift detection]
  teardown[destroy.yml every night:<br/>dev, staging, stale previews]
```

The PR stages are drawn in sequence for reading; in `pr.yml` they run as parallel jobs
that start only when their paths changed, and `CI OK` gates the merge.

Every deployment (`deploy-environment.yml`) runs the same steps: targeted apply, migrate and
seed as one-off ECS tasks, full apply, then a safe rollout and post-deploy tests.

```mermaid
flowchart LR
  apply[terraform apply] --> green[green API tasks<br/>next to blue]
  green --> hook{deploy hook:<br/>smoke test through<br/>the test listener rule}
  hook -->|fails| rb[ECS rolls back]
  hook -->|passes| switch[production rule<br/>to green]
  switch --> bake[bake time,<br/>5xx alarm watches]
  bake -->|alarm| rb
  bake --> canary[runner canary, prod:<br/>hook judges A + B,<br/>10 % for 5 minutes]
  canary -->|error alarm| rbl[alias rolls back]
  canary --> tests[smoke, Playwright with a<br/>temporary Cognito user]
```

## Testing

| Layer | What | Where |
|---|---|---|
| Unit | Go (race detector), Vitest + Testing Library, pytest for runners and Lambdas | `make test`, `make fe-check`, `make py-test` |
| Sandbox | probe programs through the runner images without network: limits, file system, environment, leftover processes | `make sandbox-suite` |
| Golden | every problem has one solution per expected verdict (AC, WA, TLE, MLE, RE, CE, ...) in Python and C++ | `tests/golden`, `make golden` |
| Integration | Testcontainers PostgreSQL, ElasticMQ and Valkey: claims, outbox, sweeper, API with an N+1 query guard | `make test-integration` |
| Contract | OpenAPI response validation in every API test, Pact SPA ↔ API, AsyncAPI for WebSocket messages | `make pact` |
| End to end | golden solutions through the whole local chain, Playwright with page objects and axe | `make e2e`, `make fe-e2e` |
| Post-deploy | Playwright against CloudFront: security headers, edge rules, signed-in submissions | `frontend/e2e/deployed` |
| Non-functional | k6 judge latency and lost submissions, worker scaling, FIS worker kill, OWASP ZAP | `nonfunctional.yml` |
| Mutation | Gremlins (Go judge logic), Stryker (SPA) with thresholds | `nightly.yml` |
| Infrastructure | `terraform test` with mocked providers, tflint, Checkov, nightly drift detection | `infra/modules/app/tests` |

## Repository layout

```
backend/            Go: cmd/api, cmd/worker, cmd/judge, cmd/seed, cmd/contest, cmd/migratediff
frontend/           React + Vite + TypeScript SPA (Monaco, typed API client, Playwright, Pact)
runners/            runner protocol (JSON Schema), sandbox core, Python and C++ Lambda images
lambdas/            Python Lambdas: ws (WebSocket), deploy-hook (rollout smoke tests)
contracts/          OpenAPI 3.1 and AsyncAPI contracts
tests/              golden problems, sandbox security, performance (k6), security (ZAP)
infra/bootstrap/    one-time account foundation: state bucket, GitHub OIDC roles, budget
infra/shared/       account-wide resources (ECR repositories)
infra/modules/      Terraform modules (app, backend, network, runner-lambda, contests, ...)
infra/envs/         dev, staging, prod stacks
infra/preview/      shared preview VPC and the per-PR stack
docs/               plan, progress, setup guide, ADRs
.github/workflows/  CI/CD pipelines
```

## Getting started

Prerequisites and the one-time AWS and GitHub setup: [docs/SETUP.md](docs/SETUP.md).

```bash
make help         # list targets
make local-up     # full stack in Docker: Postgres, Valkey, ElasticMQ, runners, API, worker (+ migrations, seed)
make fe-dev       # SPA on http://localhost:5173 (sign in with any lowercase name)
make e2e          # golden solutions through the whole chain
make fe-e2e       # Playwright browser tests + accessibility scan
make local-traces # the same stack with traces in Jaeger (http://localhost:16686)
make demo-video   # record docs/media/demo.gif (needs make local-up)
make test         # Go unit tests
make golden       # every golden solution must get its expected verdict (make runner-up first)
make sandbox-suite # runner hardening: limits, no network, read-only root, no leftovers
make judge SRC=tests/golden/a-plus-b/solutions/ac.py
make tf-validate  # validate all Terraform stacks
make check        # every local check
```

## Decisions

Architecture decision records are in [docs/adr](docs/adr), from the monorepo
([0002](docs/adr/0002-monorepo.md)) and the Lambda sandbox ([0003](docs/adr/0003-sandbox-on-lambda.md))
to preview environments ([0019](docs/adr/0019-preview-environments-shared-vpc.md)) and
blue/green with canaries ([0020](docs/adr/0020-blue-green-api-canary-runners.md)).

## Cost control

Everything is Terraform, so an environment exists only while it is needed: dev and staging
are destroyed every night, previews when their pull request closes, and nothing runs a NAT
gateway ([ADR-0005](docs/adr/0005-network-without-nat.md)). Budget alerts are part of the
bootstrap stack.
