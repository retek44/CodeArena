# The network every preview environment deploys into (ADR-0019). A VPC, its subnets,
# route tables and the internet gateway cost nothing, so this stack stays: the preview
# workflow applies it (a no-op once it exists) before each preview, and no preview
# depends on dev, which is destroyed every night.

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "codearena"
      Environment = "preview"
      ManagedBy   = "terraform"
    }
  }
}

data "aws_region" "current" {}

module "network" {
  source = "../../modules/network"

  name_prefix        = "codearena-preview"
  cidr_block         = "10.90.0.0/16"
  availability_zones = ["${data.aws_region.current.region}a", "${data.aws_region.current.region}b"]
}
