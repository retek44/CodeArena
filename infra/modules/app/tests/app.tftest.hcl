# Plan-only tests with mocked providers: no AWS account, no credentials, no cost.
# They catch what `terraform validate` cannot, such as for_each over values that are
# unknown until apply, and they pin the contract the deploy pipeline relies on.
#
#   terraform -chdir=infra/modules/app init -backend=false
#   terraform -chdir=infra/modules/app test

mock_provider "aws" {
  mock_data "aws_caller_identity" {
    defaults = { account_id = "123456789012" }
  }
  mock_data "aws_partition" {
    defaults = { partition = "aws" }
  }
  mock_data "aws_region" {
    defaults = { region = "eu-central-1", name = "eu-central-1" }
  }
  mock_data "aws_iam_policy_document" {
    defaults = { json = "{\"Version\":\"2012-10-17\",\"Statement\":[]}" }
  }
  mock_data "aws_ecr_repository" {
    defaults = { repository_url = "123456789012.dkr.ecr.eu-central-1.amazonaws.com/codearena-mock" }
  }
  mock_data "aws_vpc" {
    defaults = { cidr_block = "10.20.0.0/16" }
  }
}

mock_provider "random" {}

variables {
  env        = "dev"
  cidr_block = "10.20.0.0/16"
}

run "base_without_image_tag" {
  command = plan

  assert {
    condition     = output.deploy == null && output.site_url == null
    error_message = "Without image_tag only the base (network, queue, Cognito) may be planned."
  }
}

run "full_environment_plans" {
  command = plan

  variables {
    image_tag = "0123456789abcdef0123456789abcdef01234567"
  }

  assert {
    condition     = toset(keys(output.deploy.job_task_definitions)) == toset(["migrate", "seed"])
    error_message = "The pipeline runs the migrate and seed jobs by these names."
  }

  assert {
    condition     = length(output.deploy.task_subnet_ids) == 2
    error_message = "One-off jobs need the two public subnets."
  }
}

run "rejects_unknown_environment" {
  command = plan

  variables {
    env = "qa"
  }

  expect_failures = [var.env]
}
