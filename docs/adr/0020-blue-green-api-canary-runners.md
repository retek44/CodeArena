# ADR-0020: Blue/green API with a smoke-test hook, canary for the runner Lambdas

- Status: accepted
- Date: 2026-11-15

## Context

PLAN.md asks for safe production rollouts: ECS blue/green with a lifecycle-hook smoke
test, a Lambda canary of 10 % for 5 minutes through CodeDeploy, and automatic rollback on
alarms. Until now the API service rolled tasks over in place (circuit breaker and a 5xx
alarm), and Terraform pointed the worker straight at the unqualified runner functions, so a
broken runner image judged every submission from the first second.

Facts that shaped the design:

- ECS has native blue/green deployments (`deployment_configuration.strategy = "BLUE_GREEN"`)
  with Lambda lifecycle hooks, a test listener rule and a bake time. The older way, the
  `CODE_DEPLOY` deployment controller, takes task definition changes away from Terraform
  (every deploy becomes an AppSpec with a new task definition) and does not support the
  circuit breaker.
- The ALB accepts traffic from CloudFront only (ADR-0012). A hook cannot call it directly;
  a test listener on another port would be unreachable too.
- Lambda canaries need an alias and published versions; CodeDeploy then owns the alias.
- Runner traffic in prod is low (users plus the Synthetics canary every 30 minutes): a
  5-minute canary may see no real submission, so an alarm alone proves little.

## Decision

- **API, every environment:** native ECS blue/green. A second target group (`-api-alt`);
  the production listener rule and a test listener rule (priority 5) forward to both with
  weights that ECS moves (`ignore_changes = [action]`, otherwise Terraform would move them
  back). The test rule matches the CloudFront secret header plus `X-Codearena-Test-Traffic`
  with its own random value, so only callers that know it reach the green tasks; CloudFront
  forwards viewer headers to `/api/*` (`AllViewerExceptHostHeader`).
- **Smoke-test hook:** `lambdas/deploy-hook` runs at `POST_TEST_TRAFFIC_SHIFT`. It reads
  the site URL and the test header value from the SecureString parameter
  `/<env prefix>/deploy-hook` (written by the app module after the CDN exists, so there is
  no dependency cycle), then calls `GET /api/problems` and one problem page through
  CloudFront with the test header: green tasks, database and seed in one check. Server
  errors or wrong bodies answer `FAILED` (ECS rolls back); an unreachable site (first
  deployment while CloudFront still deploys, parameter not written yet, 502/503/504)
  answers `IN_PROGRESS` for up to 20 attempts. A rollback call is answered at once.
- **Bake time:** `api_bake_time_minutes`, 5 in prod, 0 elsewhere. The blue tasks stay that
  long; the deployment circuit breaker and the 5xx alarm (now on the whole load balancer,
  because traffic moves between two target groups) roll back to them.
- **Runners:** every image change publishes a Lambda version; the worker invokes the `live`
  alias. Without `runner_canary` Terraform moves the alias at once (dev, staging, previews).
  With it (prod) CodeDeploy owns the alias: `CodeDeployDefault.LambdaCanary10Percent5Minutes`,
  rollback on deployment failure or on the alias `Errors` alarm. The same hook Lambda is the
  `BeforeAllowTraffic` hook: it invokes the newest version with the accepted A + B solution
  of `tests/golden` and reports through `PutLifecycleEventHookExecutionStatus`; the
  deployment group is named after the function, so the hook finds it.
- **Pipeline:** `deploy-environment.yml` waits for the newest ECS service deployment and
  fails unless it is `SUCCESSFUL` (`wait-api-deployment.sh`; `services-stable` also
  succeeds after a rollback), then starts one CodeDeploy deployment per changed runner and
  waits for them (`runner-canary.sh`).

## Consequences

- A broken API image never receives production traffic: it fails the smoke test and ECS
  removes it. A broken runner image fails the hook before the alias moves.
- Deploys take longer: green tasks start next to blue (two task sets for a few minutes),
  the hook adds about a minute, prod adds 5 minutes of bake time and 5 minutes of canary.
- The API service needs two extra IAM roles (`AmazonECSInfrastructureRolePolicyForLoadBalancers`
  and one that may invoke the hook); CodeDeploy needs a service role
  (`AWSCodeDeployRoleForLambda`, plus invoke rights on the hook, because the managed
  policy only allows functions named `CodeDeployHook_*`). All carry the workload boundary.
- Old runner versions pile up (one per deploy). They cost nothing, but their images must
  stay in ECR as long as a rollback target needs them.
- Not yet verified on AWS: the first blue/green deployment of a new service (hook waiting
  for CloudFront), a forced rollback (failing hook), and the runner canary in prod.
