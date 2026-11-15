# Preview environment of one pull request (ADR-0019): the same app module as dev,
# staging and prod, inside the shared preview VPC (infra/preview/base), with the
# cost-heavy extras off. Created when the PR gets the "preview" label, destroyed when
# the PR closes or loses the label (preview.yml), or by the nightly sweep.

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "codearena"
      Environment = "pr-${var.pr_number}"
      ManagedBy   = "terraform"
    }
  }
}

data "aws_vpc" "preview" {
  tags = { Name = "codearena-preview-vpc" }
}

data "aws_subnets" "public" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.preview.id]
  }
  tags = { Name = "codearena-preview-public-*" }
}

data "aws_subnets" "isolated" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.preview.id]
  }
  tags = { Name = "codearena-preview-isolated-*" }
}

module "app" {
  source = "../../modules/app"

  env       = "pr-${var.pr_number}"
  image_tag = var.image_tag
  network = {
    vpc_id              = data.aws_vpc.preview.id
    public_subnet_ids   = sort(data.aws_subnets.public.ids)
    isolated_subnet_ids = sort(data.aws_subnets.isolated.ids)
  }

  # Short-lived and seen by a reviewer or two: no traces, one worker.
  tracing          = { enabled = false, sample_ratio = 0 }
  worker_max_count = 1
}
