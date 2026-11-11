.DEFAULT_GOAL := help
SHELL := bash

TF_DIRS := infra/bootstrap infra/shared $(wildcard infra/envs/*) $(wildcard infra/modules/*)
GO_MODULES := api worker
RUNNER_IMAGE := codearena-runner-python:local
RUNNER_URL ?= http://localhost:9000/2015-03-31/functions/function/invocations

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# --- Go -----------------------------------------------------------------------

.PHONY: lint
lint: ## Lint Go code (all modules)
	@for m in $(GO_MODULES); do echo "==> $$m"; (cd $$m && golangci-lint run ./...) || exit 1; done

.PHONY: test
test: ## Run Go unit tests with race detector (all modules)
	@for m in $(GO_MODULES); do echo "==> $$m"; (cd $$m && go test -race -cover ./...) || exit 1; done

.PHONY: run-api
run-api: ## Run the API locally
	cd api && go run ./cmd/api

.PHONY: image
image: ## Build the API container image
	docker build -t codearena-api:local api

# --- Runner -------------------------------------------------------------------

.PHONY: py-lint
py-lint: ## ruff + mypy for the Python runner
	ruff check runners
	ruff format --check runners
	mypy

.PHONY: py-test
py-test: ## Runner unit and contract tests (Linux; on Windows use py-test-docker)
	pytest -q runners/core/tests

.PHONY: py-test-docker
py-test-docker: ## Runner lint and tests inside a Linux container
	docker run --rm -v "$(CURDIR)":/src -w /src python:3.13-slim sh -c 		'pip install -q -r requirements-dev.txt && ruff check runners && mypy && pytest -q runners/core/tests'

.PHONY: runner-image
runner-image: ## Build the Python runner image
	docker build -t $(RUNNER_IMAGE) -f runners/python/Dockerfile runners

.PHONY: runner-up
runner-up: runner-image ## Start the runner on :9000, restricted like Lambda
	-docker rm -f codearena-runner >/dev/null 2>&1
	docker run -d --name codearena-runner --read-only --tmpfs /tmp:rw,size=256m --memory 1g 		-p 9000:8080 $(RUNNER_IMAGE)

.PHONY: runner-down
runner-down: ## Stop the runner container
	-docker rm -f codearena-runner

.PHONY: golden
golden: ## Golden verdict tests against the running runner (make runner-up first)
	cd worker && CODEARENA_RUNNER_URL=$(RUNNER_URL) go test -count=1 -run TestGolden -v ./internal/judge/

# Usage: make judge SRC=tests/golden/a-plus-b/solutions/ac.py [PROBLEM=tests/golden/a-plus-b]
PROBLEM ?= tests/golden/a-plus-b

.PHONY: judge
judge: ## Judge one solution locally (SRC=..., PROBLEM=...)
	cd worker && CODEARENA_RUNNER_URL=$(RUNNER_URL) go run ./cmd/judge -problem ../$(PROBLEM) -src ../$(SRC)

# --- Local stack --------------------------------------------------------------

.PHONY: local-up
local-up: ## Start local dependencies (Postgres, Valkey, API)
	docker compose up -d --build --wait

.PHONY: local-down
local-down: ## Stop local dependencies and delete their data
	docker compose down -v

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
