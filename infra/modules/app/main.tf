# One CodeArena environment. infra/envs/{dev,staging,prod} call this module with their
# own sizes, so the environments differ only in variables, never in structure.
#
# Without image_tag only the base is created (network, queue, Cognito). The pipeline
# passes the git SHA of the pushed images; then the database, runner, backend and CDN
# are created too. Destroying the environment removes everything except the shared ECR images.

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}
data "aws_region" "current" {}

locals {
  name_prefix  = "codearena-${var.env}"
  account_id   = data.aws_caller_identity.current.account_id
  region       = data.aws_region.current.region
  boundary_arn = "arn:${data.aws_partition.current.partition}:iam::${local.account_id}:policy/codearena-workload-boundary"
  deploy       = var.image_tag != null
  site_url     = local.deploy ? "https://${module.web[0].domain_name}" : null

  local_dev_urls = var.allow_localhost_sign_in ? ["http://localhost:5173"] : []

  # Contest lifecycle names: the backend derives ARNs from them, so it does not wait
  # for the state machine (which needs the backend's task definition).
  contest_machine_name   = "${local.name_prefix}-contest"
  contest_schedule_group = "${local.name_prefix}-contests"
  contest_scheduler_role = "${local.name_prefix}-contest-scheduler"

  # Languages with a runner image in ECR (build-images.yml builds codearena-runner-<lang>).
  runner_languages = ["python", "cpp"]
}

moved {
  from = module.runner_python[0]
  to   = module.runner["python"]
}

module "network" {
  source = "../network"

  name_prefix        = local.name_prefix
  cidr_block         = var.cidr_block
  availability_zones = ["${local.region}a", "${local.region}b"]
}

# Operational alarms of the environment go to one topic; an e-mail subscription is optional.
resource "aws_sns_topic" "alerts" {
  #checkov:skip=CKV_AWS_26:CloudWatch alarms cannot publish to a topic encrypted with the AWS managed key, a customer managed key costs 1 USD a month, and alarm messages hold no secrets.
  name = "${local.name_prefix}-alerts"
}

resource "aws_sns_topic_subscription" "alerts_email" {
  count = var.alert_email == "" ? 0 : 1

  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

module "submissions_queue" {
  source = "../sqs"

  name          = "${local.name_prefix}-submissions"
  alarm_actions = [aws_sns_topic.alerts.arn]
}

module "cognito" {
  source = "../cognito"

  name_prefix = local.name_prefix
  # Globally unique: the account ID keeps it free of collisions.
  domain_prefix       = "${local.name_prefix}-${local.account_id}"
  callback_urls       = [for u in concat(local.local_dev_urls, local.deploy ? [local.site_url] : []) : "${u}/auth/callback"]
  logout_urls         = [for u in concat(local.local_dev_urls, local.deploy ? [local.site_url] : []) : "${u}/"]
  deletion_protection = var.protect_data
}

# --- Compute, only with a deployed image tag ----------------------------------------

data "aws_ecr_repository" "this" {
  for_each = local.deploy ? toset(concat(
    ["codearena-api", "codearena-worker", "codearena-migrate", "codearena-seed", "codearena-contest"],
    [for lang in local.runner_languages : "codearena-runner-${lang}"],
  )) : toset([])

  name = each.key
}

# One runner function per language (one image each, see runners/).
module "runner" {
  source   = "../runner-lambda"
  for_each = local.deploy ? toset(local.runner_languages) : toset([])

  name_prefix              = local.name_prefix
  language                 = each.key
  image_uri                = "${data.aws_ecr_repository.this["codearena-runner-${each.key}"].repository_url}:${var.image_tag}"
  vpc_id                   = module.network.vpc_id
  subnet_ids               = module.network.isolated_subnet_ids
  permissions_boundary_arn = local.boundary_arn
}

# Contests: Valkey leaderboard cache, lifecycle state machine, schedules (ADR-0016).
module "contests" {
  source = "../contests"
  count  = local.deploy ? 1 : 0

  name_prefix              = local.name_prefix
  permissions_boundary_arn = local.boundary_arn
  vpc_id                   = module.network.vpc_id
  subnet_ids               = module.network.isolated_subnet_ids
  task                     = module.backend[0].contest
  state_machine_name       = local.contest_machine_name
  schedule_group           = local.contest_schedule_group
  scheduler_role_name      = local.contest_scheduler_role
}

# Live updates over WebSocket (ADR-0015).
module "realtime" {
  source = "../realtime"
  count  = local.deploy ? 1 : 0

  name_prefix              = local.name_prefix
  permissions_boundary_arn = local.boundary_arn
}

# AI hint for failed submissions (Bedrock with a guardrail, ADR-0014).
module "hint" {
  source = "../hint"
  count  = local.deploy && var.hints_enabled ? 1 : 0

  name_prefix              = local.name_prefix
  permissions_boundary_arn = local.boundary_arn
  model_id                 = var.hint_model_id
}

module "postgres" {
  source = "../postgres"
  count  = local.deploy ? 1 : 0

  name_prefix               = local.name_prefix
  vpc_id                    = module.network.vpc_id
  subnet_ids                = module.network.isolated_subnet_ids
  client_security_group_ids = module.backend[0].security_group_ids
  instance_class            = var.db_instance_class
  backup_retention_days     = var.protect_data ? 7 : 1
  deletion_protection       = var.protect_data
  skip_final_snapshot       = !var.protect_data
}

module "backend" {
  source = "../backend"
  count  = local.deploy ? 1 : 0

  name_prefix              = local.name_prefix
  vpc_id                   = module.network.vpc_id
  public_subnet_ids        = module.network.public_subnet_ids
  permissions_boundary_arn = local.boundary_arn
  api_desired_count        = var.api_desired_count
  worker_max_count         = var.worker_max_count
  runner_function_arns     = { for lang, runner in module.runner : lang => runner.function_arn }
  hint_function_arn        = one(module.hint[*].function_arn)

  valkey_url = module.contests[0].valkey_url

  contests = {
    image                = "${data.aws_ecr_repository.this["codearena-contest"].repository_url}:${var.image_tag}"
    email_from           = var.contest_email_from
    cognito_user_pool_id = module.cognito.user_pool_id
    cognito_user_pool    = module.cognito.user_pool_arn
    state_machine_arn    = "arn:${data.aws_partition.current.partition}:states:${local.region}:${local.account_id}:stateMachine:${local.contest_machine_name}"
    scheduler_group      = local.contest_schedule_group
    scheduler_role_arn   = "arn:${data.aws_partition.current.partition}:iam::${local.account_id}:role/${local.contest_scheduler_role}"
  }

  realtime = {
    ws_url    = module.realtime[0].ws_url
    table_arn = module.realtime[0].table.arn
    table     = module.realtime[0].table.name
    bus_arn   = module.realtime[0].event_bus.arn
    bus       = module.realtime[0].event_bus.name
  }

  images = {
    api     = "${data.aws_ecr_repository.this["codearena-api"].repository_url}:${var.image_tag}"
    worker  = "${data.aws_ecr_repository.this["codearena-worker"].repository_url}:${var.image_tag}"
    migrate = "${data.aws_ecr_repository.this["codearena-migrate"].repository_url}:${var.image_tag}"
    seed    = "${data.aws_ecr_repository.this["codearena-seed"].repository_url}:${var.image_tag}"
  }

  database = {
    host                = module.postgres[0].address
    name                = module.postgres[0].database
    user                = module.postgres[0].username
    password_secret_arn = module.postgres[0].password_secret_arn
  }

  cognito = {
    issuer    = module.cognito.issuer
    client_id = module.cognito.client_id
  }

  queue = {
    url                        = module.submissions_queue.queue_url
    arn                        = module.submissions_queue.queue_arn
    name                       = module.submissions_queue.queue_name
    visibility_timeout_seconds = module.submissions_queue.visibility_timeout_seconds
  }
}

module "web" {
  source = "../frontend-cdn"
  count  = local.deploy ? 1 : 0

  name_prefix              = local.name_prefix
  api_origin_domain        = module.backend[0].alb_dns_name
  api_origin_verify_header = module.backend[0].origin_verify_header
  content_security_policy = join("; ", [
    "default-src 'self'",
    "script-src 'self'",
    # Monaco injects style elements at runtime.
    "style-src 'self' 'unsafe-inline'",
    "worker-src 'self' blob:",
    "img-src 'self' data:",
    "font-src 'self' data:",
    "connect-src 'self' https://${module.cognito.login_domain} https://cognito-idp.${local.region}.amazonaws.com ${module.realtime[0].ws_origin}",
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "object-src 'none'",
  ])
}

# Dashboard of the environment, and the Synthetics canary where enabled (prod).
module "observability" {
  source = "../observability"
  count  = local.deploy ? 1 : 0

  name_prefix              = local.name_prefix
  permissions_boundary_arn = local.boundary_arn
  alerts_topic_arn         = aws_sns_topic.alerts.arn
  site_url                 = local.site_url
  canary_enabled           = var.canary_enabled

  cognito = {
    user_pool_id = module.cognito.user_pool_id
    client_id    = module.cognito.client_id
  }

  dashboard = {
    alb_arn_suffix          = module.backend[0].alb_arn_suffix
    target_group_arn_suffix = module.backend[0].api_target_group_arn_suffix
    cluster_name            = module.backend[0].cluster_name
    api_service_name        = module.backend[0].api_service_name
    worker_service_name     = module.backend[0].worker_service_name
    queue_name              = module.submissions_queue.queue_name
    dlq_name                = module.submissions_queue.dlq_name
    db_identifier           = module.postgres[0].identifier
    runner_function_names   = { for lang, runner in module.runner : lang => runner.function_name }
    worker_log_group        = module.backend[0].log_group_names["worker"]
    api_log_group           = module.backend[0].log_group_names["api"]
  }
}
