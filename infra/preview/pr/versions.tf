terraform {
  required_version = ">= 1.10.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # One state per pull request: init with -backend-config="key=preview/pr-<n>/terraform.tfstate"
  # (bucket and region as for the other stacks).
  backend "s3" {
    use_lockfile = true
    encrypt      = true
  }
}
