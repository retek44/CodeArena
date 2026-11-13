provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "codearena"
      Environment = "prod"
      ManagedBy   = "terraform"
    }
  }
}

module "app" {
  source = "../../modules/app"

  env                = "prod"
  image_tag          = var.image_tag
  alert_email        = var.alert_email
  contest_email_from = var.contest_email_from
  cidr_block         = "10.40.0.0/16"
  protect_data       = true
  api_desired_count  = 2
  worker_max_count   = 4
  canary_enabled     = true
}
