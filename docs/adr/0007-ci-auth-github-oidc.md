# ADR-0007: CI authenticates to AWS with GitHub OIDC

- Status: accepted
- Date: 2026-10-12

## Context

CI needs AWS access for `terraform plan` and deployments. Long-lived access keys in GitHub
secrets can leak and must be rotated.

## Decision

- GitHub Actions gets short-lived credentials through the account's OIDC provider for
  `token.actions.githubusercontent.com`.
- **Plan role** (`codearena-ci-plan`): `ReadOnlyAccess` plus state read and lock-file write.
  Trusted for `pull_request` and `refs/heads/main` of this repository only.
- **Deploy roles** (`codearena-ci-deploy-<env>`): trusted only for jobs that run in the GitHub
  Environment `<env>`. The `prod` environment needs a reviewer, so prod credentials cannot
  be obtained without approval.
- Deploy roles have `PowerUserAccess` plus limited IAM rights: they may create or change
  roles only with the `codearena-` prefix and **only with the workload permissions boundary**
  attached. They cannot remove the boundary, change the boundary policy or touch the CI roles.
  This blocks privilege escalation through a new admin role.
- Terraform state: S3 bucket with native locking (`use_lockfile`, Terraform ≥ 1.10).
  No DynamoDB lock table.
- The bootstrap stack (state bucket, OIDC, CI roles, budget) is applied by a human with admin
  rights and is never changed by CI.

## Consequences

- No AWS secrets in GitHub.
- Fork PRs get no OIDC token, so `terraform plan` is skipped for them.
- Every IAM role defined in later phases must set
  `permissions_boundary = <workload boundary ARN>`, otherwise the deploy fails.
- Owner and repository names in the trust policy are case-sensitive.
