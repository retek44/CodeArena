provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "codearena"
      Environment = local.env
      ManagedBy   = "terraform"
    }
  }
}

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

locals {
  env          = "dev"
  name_prefix  = "codearena-${local.env}"
  boundary_arn = "arn:${data.aws_partition.current.partition}:iam::${data.aws_caller_identity.current.account_id}:policy/codearena-workload-boundary"
}

module "network" {
  source = "../../modules/network"

  name_prefix        = local.name_prefix
  cidr_block         = "10.20.0.0/16"
  availability_zones = ["${var.region}a", "${var.region}b"]
}

module "cognito" {
  source = "../../modules/cognito"

  name_prefix = local.name_prefix
  # Globally unique: the account ID keeps it free of collisions.
  domain_prefix = "${local.name_prefix}-${data.aws_caller_identity.current.account_id}"
  callback_urls = ["http://localhost:5173/auth/callback"]
  logout_urls   = ["http://localhost:5173/"]
}

# The function needs an image in ECR, so it is created only after CI pushed one
# and the tag is passed in (-var runner_python_image_tag=<git sha>).
data "aws_ecr_repository" "runner_python" {
  count = var.runner_python_image_tag == null ? 0 : 1

  name = "codearena-runner-python"
}

module "runner_python" {
  source = "../../modules/runner-lambda"
  count  = var.runner_python_image_tag == null ? 0 : 1

  name_prefix              = local.name_prefix
  language                 = "python"
  image_uri                = "${data.aws_ecr_repository.runner_python[0].repository_url}:${var.runner_python_image_tag}"
  vpc_id                   = module.network.vpc_id
  subnet_ids               = module.network.isolated_subnet_ids
  permissions_boundary_arn = local.boundary_arn
}
