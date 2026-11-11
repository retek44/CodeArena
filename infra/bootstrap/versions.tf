terraform {
  required_version = ">= 1.10.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # The first apply used local state (the bucket did not exist yet); the state was then
  # migrated here with `terraform init -migrate-state`. See docs/SETUP.md.
  backend "s3" {
    bucket       = "codearena-tfstate-379269296137-eu-central-1"
    key          = "bootstrap/terraform.tfstate"
    region       = "eu-central-1"
    use_lockfile = true
    encrypt      = true
  }
}
