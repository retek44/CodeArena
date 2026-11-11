# CLAUDE.md

Context for AI assistants that continue work on this repository.

## Project

CodeArena: an online judge on AWS, built as a portfolio project for developer and
QA/test automation roles. The CI/CD pipeline and its test layers are as important as the
application code.

- Full plan: `docs/PLAN.md`
- What is done and what is next: `docs/PROGRESS.md` (read this first)
- Manual AWS/GitHub steps: `docs/SETUP.md`
- Decisions: `docs/adr/` (ADR-0004 hybrid data access, ADR-0007 CI auth and IAM boundary)

## How to continue

1. Read `docs/PROGRESS.md` and pick the first open item of the current phase.
2. Keep each change PR-sized and match the existing style.
3. Update `docs/PROGRESS.md` in the same change. Add an ADR for any new architecture decision.
4. Verify locally before you report done (see below).

## Conventions

- Code, comments, docs and commit messages are in English.
- Conventional Commits; PRs are squash-merged, so the PR title is the commit message.
- Go: standard library first (`net/http` routing, `log/slog` JSON logs), table-driven tests,
  `t.Parallel()`, `httptest.NewRequestWithContext(t.Context(), ...)`.
- Data access: Ent for CRUD, sqlc for hot paths (`RETURNING`, window functions, bulk, locking),
  Atlas versioned migrations. Never auto-migrate. Migrations must be backward compatible.
- Terraform:
  - Resource names `codearena-<env>-<name>`; preview `codearena-pr-<n>-<name>`.
  - Region `eu-central-1`, AWS provider `~> 6.0`, Terraform `>= 1.10`, S3 backend with `use_lockfile`.
  - Every IAM role created outside `infra/bootstrap` must set
    `permissions_boundary` to the `codearena-workload-boundary` policy, or CI deploy fails.
  - New services used by workloads must be added to `workload_services` in
    `infra/modules/github-oidc/variables.tf` (bootstrap change, applied by a human).
  - Checkov exceptions are inline `#checkov:skip=<id>:<reason>` next to the resource.
- GitHub Actions: pin every action to a full commit SHA with a `# vX.Y.Z` comment;
  pass `${{ }}` values into `run:` through `env:`, not inline.
- Cost: no NAT Gateway, no EKS by default, arm64 everywhere, everything destroyable.
- Runner protocol changes: update `runners/protocol/*.schema.json`, `runners/core/codearena_runner/protocol.py`
  and `worker/internal/protocol` together; the pytest contract tests compare parser and schema.
- Golden tests: a solution file name starts with its expected verdict (`ac`, `wa`, `tle`, `mle`, `re`, `ce`).
- AWS: account `379269296137`, CLI profile `CodeArena`, region `eu-central-1`. Do not use the
  "Fresh Deploy" account (207871833442, profile `codearena`): its SCPs block almost every service.

## Local verification

Go, Terraform and the linters may not be installed on the host. Docker works for all of them
(run from the repo root in Git Bash with `MSYS_NO_PATHCONV=1` and `W="$(pwd -W)"`):

```bash
docker run --rm -v "$W":/work -w /work --entrypoint sh hashicorp/terraform:1.16.5 -c 'terraform fmt -check -recursive infra'
docker run --rm -v "$W":/data -w /data --entrypoint sh ghcr.io/terraform-linters/tflint:v0.64.0 -c 'tflint --init --config /data/.tflint.hcl && tflint --recursive --config /data/.tflint.hcl'
docker run --rm -v "$W":/tf -w /tf bridgecrew/checkov:latest -d infra --config-file .checkov.yaml
docker run --rm -v "$W":/src -w /src/api golang:1.26-alpine go test ./...      # same for /src/worker
docker run --rm -v "$W":/src -w /src/api golangci/golangci-lint:v2.14.0 golangci-lint run ./...   # same for worker
docker run --rm -v "$W":/src -w /src python:3.13-slim sh -c 'pip install -q -r requirements-dev.txt && ruff check runners && mypy && pytest -q runners/core/tests'
make runner-up && make golden   # golden verdict tests against the runner container
TESTCONTAINERS_RYUK_DISABLED=true make test-integration   # Postgres integration tests (Ryuk fails on Docker Desktop)
docker run --rm -v "$W":/tmp/repo -w /tmp/repo stoplight/spectral:6.17.1 lint --fail-severity=warn contracts/openapi.yaml
docker run --rm -v "$W":/repo -w /repo rhysd/actionlint:1.7.12
docker run --rm -v "$W":/src -w /src semgrep/semgrep:1.180.0 semgrep scan --error --metrics=off --config p/golang --config p/python --config p/dockerfile --config p/terraform --config p/github-actions --config p/secrets
```

Remove `.terraform/` directories after validating in Docker; keep `.terraform.lock.hcl` of root stacks.
