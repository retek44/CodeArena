# ADR-0012: CloudFront in front of the SPA and the API, no custom domain

- Status: accepted
- Date: 2026-11-13

## Context

The SPA (static files) and the API (ECS behind an ALB) should share one origin: no CORS,
and one place for TLS and security headers. The project has no registered domain, so
there is no ACM certificate for the ALB.

## Decision

- One CloudFront distribution per environment: `/*` from a private S3 bucket (Origin
  Access Control), `/api/*` to the ALB with caching disabled.
- Viewers use HTTPS with the default `*.cloudfront.net` certificate; HTTP is redirected.
- CloudFront reaches the ALB over HTTP. The ALB is protected twice:
  - its security group allows only the CloudFront origin-facing managed prefix list;
  - a listener rule forwards only requests with the secret `X-Origin-Verify` header
    (random per environment, stored in Terraform state); anything else gets 403.
- Response headers policy on every behavior: CSP (`script-src 'self'`; Monaco needs
  inline styles and same-origin workers), HSTS with preload, `X-Frame-Options: DENY`,
  `nosniff`, strict referrer policy.
- A CloudFront Function rewrites client-side routes (paths without a file extension)
  to `/index.html`.

## Consequences

- No domain or certificate cost; one URL per environment.
- The CloudFront→ALB hop is plain HTTP over the AWS network. With a custom domain the
  ALB gets an ACM certificate and the origin switches to HTTPS; nothing else changes.
- Checkov and Semgrep flag the HTTP listener and the default certificate; the findings
  are suppressed next to the resources with a reference to this ADR.
