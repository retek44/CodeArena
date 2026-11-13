# Account-wide resources used by every environment. Deployed by CI with the "shared" role.

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "codearena"
      Environment = "shared"
      ManagedBy   = "terraform"
    }
  }
}

locals {
  repositories = toset([
    "codearena-api",
    "codearena-worker",
    "codearena-runner-python",
    "codearena-migrate",
    "codearena-seed",
  ])
}

module "ecr" {
  source   = "../modules/ecr-repository"
  for_each = local.repositories

  name = each.key
}
