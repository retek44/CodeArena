# ADR-0005: Network without a NAT Gateway

- Status: accepted
- Date: 2026-10-12

## Context

A NAT Gateway costs about 32 USD per month per AZ plus data processing, which is more than
the rest of a demo environment. Interface VPC endpoints for ECR, Logs, SQS and Secrets
Manager cost about 7 USD per month each per AZ.

## Decision

- **Public subnets:** ALB and Fargate tasks. Tasks get a public IP to pull images and call
  AWS APIs. Their security group allows inbound traffic only from the ALB security group.
- **Isolated subnets** (no route to the internet): RDS, ElastiCache, runner Lambdas.
- Gateway endpoints for S3 and DynamoDB (free) where private resources need them.
- Optional later: `fck-nat` on a `t4g.nano` instance (about 3 USD per month) if a private
  workload needs outbound internet.

## Consequences

- Near-zero network cost.
- Fargate tasks have public IPs. Inbound is still blocked by security groups, but this is
  weaker defense in depth than private subnets. Acceptable for a portfolio project;
  production would use private subnets with NAT or endpoints.
