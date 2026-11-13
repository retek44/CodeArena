# CodeArena

A cloud-native online judge on AWS. Solve programming problems in the browser, submit code,
watch per-test-case results arrive in real time, and compete in contests with a live
leaderboard.

> **Status:** phase 0 (foundation). See [docs/PROGRESS.md](docs/PROGRESS.md).

## Highlights

- **Sandboxed execution:** untrusted code runs in network-isolated Lambda functions
  (Firecracker microVMs) with CPU, memory, process and file limits.
- **Queue-driven judge:** SQS + Go workers on ECS Fargate, idempotent at-least-once
  processing, autoscaling on queue backlog.
- **Real time:** verdicts and leaderboards pushed over API Gateway WebSockets.
- **Contests:** Step Functions + EventBridge Scheduler, Valkey sorted-set leaderboards with freeze.
- **AI hints:** Amazon Bedrock with guardrails and automated LLM evaluation.
- **CI/CD:** GitHub Actions with OIDC (no AWS keys), per-PR preview environments, contract,
  integration, E2E, performance, DAST and chaos stages, signed images, blue/green and canary
  deployments with alarm-based rollback.
- **Infrastructure as code:** modular Terraform, Checkov, tflint, Terratest, drift detection.

## Architecture

See [docs/PLAN.md](docs/PLAN.md#2-architecture) and the decision records in [docs/adr](docs/adr).

## Repository layout

```
backend/            Go module: cmd/api (HTTP API), cmd/judge (local judge CLI), worker (phase 3)
runners/            runner protocol (JSON Schema), Python runner Lambda image
tests/golden/       problems with one solution per expected verdict
infra/bootstrap/    one-time account foundation: state bucket, GitHub OIDC roles, budget
infra/shared/       account-wide resources (ECR repositories)
infra/modules/      reusable Terraform modules (network, runner-lambda, ecr-repository, ...)
infra/envs/         dev, staging, prod stacks
docs/               plan, progress, setup guide, ADRs
.github/workflows/  CI/CD pipelines
```

## Getting started

Prerequisites and the one-time AWS and GitHub setup: [docs/SETUP.md](docs/SETUP.md).

```bash
make help        # list targets
make test        # Go unit tests (api, worker)
make runner-up   # build and start the Python runner on :9000
make golden      # every golden solution must get its expected verdict
make judge SRC=tests/golden/a-plus-b/solutions/ac.py
make local-up    # Postgres, Valkey and the API in Docker
make tf-validate # validate all Terraform stacks
```

Check the API:

```bash
curl http://localhost:8080/healthz
```

## Cost control

Everything is Terraform, so an environment is created for a session and destroyed after it
(`make up ENV=dev`, `make down ENV=dev`). Budget alerts are part of the bootstrap stack.
