provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "codearena"
      Environment = "staging"
      ManagedBy   = "terraform"
    }
  }
}

module "app" {
  source = "../../modules/app"

  env                = "staging"
  image_tag          = var.image_tag
  alert_email        = var.alert_email
  contest_email_from = var.contest_email_from
  cidr_block         = "10.30.0.0/16"

}
