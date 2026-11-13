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
}

module "network" {
  source = "../network"

  name_prefix        = local.name_prefix
  cidr_block         = var.cidr_block
  availability_zones = ["${local.region}a", "${local.region}b"]
}

module "submissions_queue" {
  source = "../sqs"

  name = "${local.name_prefix}-submissions"
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
  for_each = local.deploy ? toset(["codearena-api", "codearena-worker", "codearena-migrate", "codearena-runner-python"]) : toset([])

  name = each.key
}

module "runner_python" {
  source = "../runner-lambda"
  count  = local.deploy ? 1 : 0

  name_prefix              = local.name_prefix
  language                 = "python"
  image_uri                = "${data.aws_ecr_repository.this["codearena-runner-python"].repository_url}:${var.image_tag}"
  vpc_id                   = module.network.vpc_id
  subnet_ids               = module.network.isolated_subnet_ids
  permissions_boundary_arn = local.boundary_arn
}

module "postgres" {
  source = "../postgres"
  count  = local.deploy ? 1 : 0

  name_prefix               = local.name_prefix
  vpc_id                    = module.network.vpc_id
  subnet_ids                = module.network.isolated_subnet_ids
  client_security_group_ids = values(module.backend[0].security_group_ids)
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
  runner_function_arn      = module.runner_python[0].function_arn

  images = {
    api     = "${data.aws_ecr_repository.this["codearena-api"].repository_url}:${var.image_tag}"
    worker  = "${data.aws_ecr_repository.this["codearena-worker"].repository_url}:${var.image_tag}"
    migrate = "${data.aws_ecr_repository.this["codearena-migrate"].repository_url}:${var.image_tag}"
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
    "connect-src 'self' https://${module.cognito.login_domain} https://cognito-idp.${local.region}.amazonaws.com",
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "object-src 'none'",
  ])
}
