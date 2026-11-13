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

run "alerts_without_email" {
  command = plan

  assert {
    condition     = length(aws_sns_topic_subscription.alerts_email) == 0
    error_message = "Without alert_email no subscription may be created (CI passes an empty string)."
  }
}

run "alerts_with_email" {
  command = plan

  variables {
    alert_email = "alerts@example.com"
  }

  assert {
    condition     = length(aws_sns_topic_subscription.alerts_email) == 1
    error_message = "alert_email subscribes to the alerts topic."
  }
}

run "one_runner_per_language" {
  command = plan

  variables {
    image_tag = "0123456789abcdef0123456789abcdef01234567"
  }

  assert {
    condition     = toset(keys(module.runner)) == toset(["python", "cpp"])
    error_message = "Every language needs its runner function (the worker routes by language)."
  }
}

run "canary_only_when_enabled" {
  command = plan

  variables {
    image_tag      = "0123456789abcdef0123456789abcdef01234567"
    canary_enabled = true
  }

  assert {
    condition     = module.observability[0].canary_name == "ca-dev-journey"
    error_message = "The canary name must fit the 21-character Synthetics limit."
  }
}

run "no_canary_by_default" {
  command = plan

  variables {
    image_tag = "0123456789abcdef0123456789abcdef01234567"
  }

  assert {
    condition     = module.observability[0].canary_name == null
    error_message = "The canary costs money per run; only prod enables it."
  }
}

run "hint_by_default" {
  command = plan

  variables {
    image_tag = "0123456789abcdef0123456789abcdef01234567"
  }

  assert {
    condition     = output.deploy.hint_function_name == "codearena-dev-hint"
    error_message = "The hint Lambda is part of every deployed environment unless disabled."
  }

  assert {
    condition     = module.hint[0].model_id == "eu.anthropic.claude-haiku-5-5"
    error_message = "Hints use the EU cross-region inference profile (data stays in the EU)."
  }
}

run "no_hint_when_disabled" {
  command = plan

  variables {
    image_tag     = "0123456789abcdef0123456789abcdef01234567"
    hints_enabled = false
  }

  assert {
    condition     = length(module.hint) == 0 && output.deploy.hint_function_name == null
    error_message = "hints_enabled = false must remove the hint Lambda and the guardrail."
  }
}

run "live_updates_wired" {
  command = plan

  variables {
    image_tag = "0123456789abcdef0123456789abcdef01234567"
  }

  assert {
    condition     = module.realtime[0].event_bus.name == "codearena-dev" && module.realtime[0].table.name == "codearena-dev-ws"
    error_message = "Each environment has its own event bus and WebSocket table."
  }

  assert {
    condition     = toset(keys(module.realtime[0].function_names)) == toset(["ws", "push"])
    error_message = "The WebSocket routes and the push need their functions."
  }
}

run "contest_lifecycle_wired" {
  command = plan

  variables {
    image_tag = "0123456789abcdef0123456789abcdef01234567"
  }

  assert {
    condition     = module.contests[0].schedule_group == "codearena-dev-contests"
    error_message = "Contest schedules live in one group per environment."
  }

  assert {
    condition     = module.backend[0].contest.task_family == "codearena-dev-contest"
    error_message = "The state machine runs the contest lifecycle task."
  }
}
