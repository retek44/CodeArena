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
    condition     = toset(keys(module.runner)) == toset(["python", "cpp", "java", "javascript", "go"])
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

run "tracing_sidecars_by_default" {
  command = plan

  variables {
    image_tag = "0123456789abcdef0123456789abcdef01234567"
  }

  assert {
    condition = alltrue([
      for service in ["api", "worker"] :
      length(module.backend[0].tracing_sidecars[service]) == 1 &&
      startswith(module.backend[0].tracing_sidecars[service][0], "public.ecr.aws/aws-observability/aws-otel-collector:")
    ])
    error_message = "The API and the worker get an ADOT collector sidecar."
  }
}

run "tracing_off" {
  command = plan

  variables {
    image_tag = "0123456789abcdef0123456789abcdef01234567"
    tracing   = { enabled = false, sample_ratio = 1 }
  }

  assert {
    condition     = alltrue([for containers in values(module.backend[0].tracing_sidecars) : length(containers) == 0])
    error_message = "Without tracing there is no collector sidecar."
  }
}

# Preview environments (ADR-0019) deploy into the shared preview VPC.
run "preview_in_existing_network" {
  command = plan

  variables {
    env        = "pr-42"
    cidr_block = null
    image_tag  = "0123456789abcdef0123456789abcdef01234567"
    network = {
      vpc_id              = "vpc-0preview"
      public_subnet_ids   = ["subnet-0pub-a", "subnet-0pub-b"]
      isolated_subnet_ids = ["subnet-0iso-a", "subnet-0iso-b"]
    }
  }

  assert {
    condition     = length(module.network) == 0
    error_message = "A preview creates no VPC of its own."
  }

  assert {
    condition     = toset(output.deploy.task_subnet_ids) == toset(["subnet-0pub-a", "subnet-0pub-b"])
    error_message = "Preview tasks run in the shared VPC's public subnets."
  }

  assert {
    condition     = module.backend[0].cluster_name == "codearena-pr-42"
    error_message = "Preview resources carry the codearena-pr-<n> prefix."
  }
}

run "needs_a_network" {
  command = plan

  variables {
    cidr_block = null
  }

  expect_failures = [var.network]
}

run "api_blue_green_with_smoke_test_hook" {
  command = plan

  variables {
    image_tag = "0123456789abcdef0123456789abcdef01234567"
  }

  assert {
    condition     = module.backend[0].api_deployment.strategy == "BLUE_GREEN"
    error_message = "The API deploys blue/green in every environment (ADR-0020)."
  }

  assert {
    condition     = join(",", module.backend[0].api_deployment.hook_stages) == "POST_TEST_TRAFFIC_SHIFT"
    error_message = "The deploy hook smoke-tests green after test traffic reaches it, before production traffic moves."
  }

  assert {
    condition     = module.backend[0].api_deployment.bake_time_minutes == 0
    error_message = "Only prod keeps the old tasks for a bake time."
  }

  assert {
    condition     = aws_ssm_parameter.deploy_hook[0].name == "/codearena-dev/deploy-hook"
    error_message = "The hook reads the site and the test header from this parameter."
  }

  assert {
    condition     = output.deploy.runner_canary == null && length(aws_codedeploy_app.runner) == 0
    error_message = "Without runner_canary Terraform moves the runner aliases; no CodeDeploy."
  }
}

run "runner_canary_when_enabled" {
  command = plan

  variables {
    env                   = "prod"
    image_tag             = "0123456789abcdef0123456789abcdef01234567"
    runner_canary         = true
    api_bake_time_minutes = 5
  }

  assert {
    condition = (
      output.deploy.runner_canary.application == "codearena-prod-runners" &&
      output.deploy.runner_canary.hook == "codearena-prod-deploy-hook"
    )
    error_message = "The pipeline needs the CodeDeploy application and the hook name."
  }

  assert {
    condition = {
      for lang, r in output.deploy.runner_canary.runners : lang => r.deployment_group
      } == {
      python     = "codearena-prod-runner-python"
      cpp        = "codearena-prod-runner-cpp"
      java       = "codearena-prod-runner-java"
      javascript = "codearena-prod-runner-javascript"
      go         = "codearena-prod-runner-go"
    }
    error_message = "One deployment group per runner, named after the function (the hook relies on it)."
  }

  assert {
    condition     = module.backend[0].api_deployment.bake_time_minutes == 5
    error_message = "api_bake_time_minutes reaches the ECS service."
  }
}

run "rejects_a_too_long_bake_time" {
  command = plan

  variables {
    api_bake_time_minutes = 1441
  }

  expect_failures = [var.api_bake_time_minutes]
}

run "rejects_a_malformed_global_submission_limit" {
  command = plan

  variables {
    submission_global_limit = "300 per hour"
  }

  expect_failures = [var.submission_global_limit]
}
