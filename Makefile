.DEFAULT_GOAL := help
SHELL := bash

TF_DIRS := infra/bootstrap infra/shared $(wildcard infra/envs/*) $(wildcard infra/modules/*)
RUNNER_IMAGE := codearena-runner-python:local
RUNNER_URL ?= http://localhost:9000/2015-03-31/functions/function/invocations
RUNNER_IMAGE_CPP := codearena-runner-cpp:local
RUNNER_URL_CPP ?= http://localhost:9001/2015-03-31/functions/function/invocations

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# --- Go -----------------------------------------------------------------------

.PHONY: lint
lint: ## Lint Go code
	cd backend && golangci-lint run ./...

.PHONY: test
test: ## Run Go unit tests with race detector
	cd backend && go test -race -cover ./...

.PHONY: test-integration
test-integration: ## Integration tests with Testcontainers (needs Docker)
	cd backend && go test -tags integration -race -count=1 ./internal/...

.PHONY: pact
pact: ## Pact: SPA consumer tests, then provider verification in a Linux container (cgo + Pact FFI)
	cd frontend && PACT_DO_NOT_TRACK=true npm run test:pact
	docker run --rm -v "$(CURDIR):/repo" -v codearena-gomod:/go/pkg/mod -v /var/run/docker.sock:/var/run/docker.sock \
		-e TESTCONTAINERS_RYUK_DISABLED=true -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal -e CGO_ENABLED=1 \
		-e PACT_DO_NOT_TRACK=true -w /repo/backend golang:1.26-bookworm sh -c '\
		go run github.com/pact-foundation/pact-go/v2@v2.9.0 install -d /usr/local/lib && \
		mkdir -p /var/tmp/gobuild && GOTMPDIR=/var/tmp/gobuild \
		go test -tags integration,pact -count=1 -run TestPactProvider ./internal/httpapi'

.PHONY: run-api
run-api: ## Run the API locally
	cd backend && go run ./cmd/api

.PHONY: image
image: ## Build the API and worker container images
	docker build -t codearena-api:local --build-arg CMD=api backend
	docker build -t codearena-worker:local --build-arg CMD=worker backend

# --- Database (ADR-0004) -----------------------------------------------------

ATLAS_IMAGE := arigaio/atlas:1.3.0
ATLAS_COMMUNITY_IMAGE := arigaio/atlas:1.3.0-community
SQLC_IMAGE := sqlc/sqlc:1.31.1
# From inside a container, the compose Postgres is reachable on the host.
DOCKER_DB_HOST := host.docker.internal:5432

.PHONY: db-up
db-up: ## Start Postgres (with the atlas_dev database for diffs)
	docker compose up -d --wait postgres

.PHONY: ent
ent: ## Regenerate the Ent client after a schema change
	cd backend && go generate ./internal/store/ent

# Usage: make migration NAME=add_contests
.PHONY: migration
migration: ## Generate a versioned migration from the Ent schema (NAME=...)
	cd backend && go run ./cmd/migratediff $(NAME)

.PHONY: sqlc
sqlc: ## Regenerate sqlc code from backend/internal/store/queries
	docker run --rm -v "$(CURDIR)/backend":/src -w /src $(SQLC_IMAGE) generate

.PHONY: migrate-apply
migrate-apply: ## Apply migrations to the local codearena database
	docker run --rm -v "$(CURDIR)/backend/migrations":/migrations $(ATLAS_IMAGE) migrate apply \
		--dir file:///migrations \
		--url "postgres://codearena:codearena@$(DOCKER_DB_HOST)/codearena?sslmode=disable"

.PHONY: migrate-validate
migrate-validate: ## Check migration files against atlas.sum
	docker run --rm -v "$(CURDIR)/backend/migrations":/migrations $(ATLAS_IMAGE) migrate validate \
		--dir file:///migrations \
		--dev-url "postgres://codearena:codearena@$(DOCKER_DB_HOST)/atlas_dev?sslmode=disable"

# Usage: make migrate-lint LATEST=1 (number of newest migrations to check)
.PHONY: migrate-lint
migrate-lint: ## Lint the newest migrations (destructive / data-dependent changes fail)
	docker run --rm -v "$(CURDIR)/backend":/backend $(ATLAS_COMMUNITY_IMAGE) migrate lint \
		--config file:///backend/lint.atlas.hcl --dir file:///backend/migrations \
		--dev-url "postgres://codearena:codearena@$(DOCKER_DB_HOST)/atlas_dev?sslmode=disable" \
		--latest $(or $(LATEST),1)

# --- Runner -------------------------------------------------------------------

.PHONY: py-lint
py-lint: ## ruff + mypy for the Python runner and the hint Lambda
	ruff check runners lambdas tests/llm-eval
	ruff format --check runners lambdas tests/llm-eval
	mypy

.PHONY: py-test
py-test: ## Runner unit and contract tests (Linux; on Windows use py-test-docker)
	pytest -q runners/core/tests lambdas/hint/tests lambdas/ws/tests tests/llm-eval

.PHONY: py-test-docker
py-test-docker: ## Runner lint and tests inside a Linux container
	docker run --rm -v "$(CURDIR)":/src -w /src python:3.13-slim sh -c \
		'pip install -q -r requirements-dev.txt && ruff check runners lambdas tests/llm-eval && mypy && pytest -q runners/core/tests lambdas/hint/tests lambdas/ws/tests tests/llm-eval'

.PHONY: runner-image
runner-image: ## Build the Python and C++ runner images
	docker build -t $(RUNNER_IMAGE) -f runners/python/Dockerfile runners
	docker build -t $(RUNNER_IMAGE_CPP) -f runners/cpp/Dockerfile runners

.PHONY: runner-up
runner-up: runner-image ## Start the runner on :9000, restricted like Lambda
	-docker rm -f codearena-runner >/dev/null 2>&1
	docker run -d --name codearena-runner --read-only --tmpfs /tmp:rw,size=256m --memory 1g \
		-p 9000:8080 $(RUNNER_IMAGE)
	-docker rm -f codearena-runner-cpp >/dev/null 2>&1
	# The C++ program runs from /tmp (allowed in Lambda); Docker tmpfs is noexec by default.
	docker run -d --name codearena-runner-cpp --read-only --tmpfs /tmp:rw,exec,size=256m --memory 1g \
		-p 9001:8080 $(RUNNER_IMAGE_CPP)

.PHONY: runner-down
runner-down: ## Stop the runner containers
	-docker rm -f codearena-runner codearena-runner-cpp

.PHONY: golden
golden: ## Golden verdict tests against the running runner (make runner-up first)
	cd backend && CODEARENA_RUNNER_URL=$(RUNNER_URL) CODEARENA_RUNNER_URL_CPP=$(RUNNER_URL_CPP) go test -count=1 -run TestGolden -v ./internal/judge/

# Usage: make judge SRC=tests/golden/a-plus-b/solutions/ac.py [PROBLEM=tests/golden/a-plus-b]
PROBLEM ?= tests/golden/a-plus-b

.PHONY: judge
judge: ## Judge one solution locally (SRC=..., PROBLEM=...)
	cd backend && CODEARENA_RUNNER_URL=$(RUNNER_URL) CODEARENA_RUNNER_URL_CPP=$(RUNNER_URL_CPP) go run ./cmd/judge -problem ../$(PROBLEM) -src ../$(SRC)

# --- Frontend -----------------------------------------------------------------

.PHONY: fe-install
fe-install: ## Install frontend dependencies
	cd frontend && npm ci

.PHONY: fe-dev
fe-dev: ## Vite dev server on :5173 (proxies /api to the local stack)
	cd frontend && npm run dev

.PHONY: fe-check
fe-check: ## Frontend: API types, lint, type check, unit tests, build
	cd frontend && npm run api:types && npm run lint && npm run typecheck && npm test && npm run build

.PHONY: fe-e2e
fe-e2e: ## Playwright browser tests against the local stack (make local-up first)
	cd frontend && npx playwright test

# --- Local stack --------------------------------------------------------------

.PHONY: local-up
local-up: ## Start the full local stack (Postgres, ElasticMQ, runner, API, worker; migrations and seed)
	docker compose up -d --build --wait

.PHONY: local-traces
local-traces: ## Local stack with tracing: spans go to Jaeger (http://localhost:16686)
	OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318 docker compose --profile tracing up -d --build --wait

.PHONY: e2e
e2e: ## End-to-end test against the local stack (make local-up first)
	cd backend && go test -tags e2e -count=1 -v ./e2e/

.PHONY: local-down
local-down: ## Stop local dependencies and delete their data
	docker compose --profile tracing down -v

# --- Terraform ----------------------------------------------------------------

.PHONY: tf-fmt
tf-fmt: ## Format Terraform code
	terraform fmt -recursive infra

.PHONY: tf-validate
tf-validate: ## Validate every Terraform stack and module (no backend)
	@for d in $(TF_DIRS); do \
		echo "==> $$d"; \
		terraform -chdir=$$d init -backend=false -input=false >/dev/null && terraform -chdir=$$d validate || exit 1; \
	done

.PHONY: tf-lint
tf-lint: ## tflint + Checkov
	tflint --init --config "$(CURDIR)/.tflint.hcl"
	tflint --recursive --config "$(CURDIR)/.tflint.hcl"
	checkov -d infra --config-file .checkov.yaml

# Usage: make up ENV=dev  /  make down ENV=dev   (application stacks, from phase 1)
ENV ?= dev

.PHONY: up
up: ## Deploy an environment (ENV=dev|staging|prod)
	terraform -chdir=infra/envs/$(ENV) init -input=false -backend-config=../../backend.hcl
	terraform -chdir=infra/envs/$(ENV) apply

.PHONY: down
down: ## Destroy an environment (ENV=dev|staging|prod)
	terraform -chdir=infra/envs/$(ENV) init -input=false -backend-config=../../backend.hcl
	terraform -chdir=infra/envs/$(ENV) destroy

# --- All ----------------------------------------------------------------------

.PHONY: check
check: lint test py-lint py-test tf-validate tf-lint ## Run every local check
