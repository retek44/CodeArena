terraform {
  required_version = ">= 1.10.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # bucket and region come from -backend-config (infra/backend.hcl locally, repo variables in CI).
  backend "s3" {
    key          = "envs/staging/terraform.tfstate"
    use_lockfile = true
    encrypt      = true
  }
}
