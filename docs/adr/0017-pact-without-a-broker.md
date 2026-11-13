# ADR-0017: Pact contract tests without a broker

- Status: accepted
- Date: 2026-11-13

## Context

PLAN.md asks for Pact between the SPA and the API and for `pact can-i-deploy` before
staging. The API responses are already validated against `contracts/openapi.yaml` in
the integration tests, but that checks the provider against its own document, not against
what the SPA actually sends and reads. A Pact Broker (or PactFlow) is one more service to
run and pay for; frontend and backend live in one repository and deploy together.

## Decision

- **Consumer:** the SPA's request functions (`frontend/src/api/requests.ts`, also used by the
  hooks) run against the Pact mock server (`npm run test:pact`, Pact JS, specification V4).
  The test writes `frontend/pacts/codearena-spa-codearena-api.json`, which is committed.
  Matchers keep the contract to what the SPA relies on (types, enums it branches on).
- **Provider:** `backend/internal/httpapi/pact_provider_test.go` (build tags
  `integration,pact`) verifies the file with pact-go against the real handler. Each
  provider state starts from a fresh database copied from the migrated template, so
  interactions do not affect each other. Values created by the provider (submission IDs)
  reach the contract through provider-state injected values. The request filter maps the
  SPA's bearer token to a development user.
- **No broker:** the CI job `contract` runs both sides on the same commit and fails when the
  committed pact differs from what the consumer tests write. A change that breaks the
  contract cannot merge, which is what `can-i-deploy` would check for a single deployable.
- The provider test needs cgo and the Pact FFI library, so it has its own build tag and is
  not part of `make test-integration`; `make pact` runs both sides locally (provider in a
  Linux container).

## Consequences

- Contract drift shows up in the pull request that causes it, on either side.
- If frontend and API ever deploy independently, add a broker and `can-i-deploy` then.
- The Go build gains pact-go (test-only import; it does not reach the binaries).
