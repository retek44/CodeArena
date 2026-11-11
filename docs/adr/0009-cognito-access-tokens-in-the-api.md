# ADR-0009: Verify Cognito access tokens in the API

- Status: accepted
- Date: 2026-11-11

## Context

Users sign in with Amazon Cognito. Requests reach the Go API through CloudFront and an
ALB (ADR-0005), so there is no API Gateway authorizer in front of the REST routes.

## Decision

- The React SPA uses the authorization code flow with PKCE against a public app client
  (no client secret in the browser). Essentials tier: 10 000 monthly active users free.
- The API verifies every request itself (`internal/auth`):
  - only RS256, keys from the user pool JWKS (cached, refreshed when an unknown `kid` appears);
  - `iss` must be the user pool, `exp` is required, `iat` must not be in the future;
  - `token_use` must be `access` (ID tokens are rejected) and `client_id` must be the SPA client.
- `AUTH_MODE` has no default. `dev` trusts an `X-Dev-User` header for local work and tests;
  AWS deployments set `cognito`. A missing value stops the service at startup.
- The user row is created on the first authenticated write (upsert on the Cognito `sub`).

## Consequences

- One code path for authentication, testable without AWS: the unit tests serve their own
  JWKS and cover expired tokens, wrong issuer or client, ID tokens, unknown keys,
  `alg: none` and HS256 key confusion.
- The API needs outbound HTTPS to Cognito for the JWKS (Fargate tasks have it, ADR-0005).
- With `username_attributes = ["email"]` the `username` claim is an opaque ID; a display
  name (`preferred_username`) can be added later.
