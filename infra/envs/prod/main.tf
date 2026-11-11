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

locals {
  env         = "prod"
  name_prefix = "codearena-${local.env}"
}

# Phase 1+ wires the application modules here (network, data, api, judge, ...).
