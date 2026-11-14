# Runner Lambda for one language (ADR-0003).
# Network isolation: isolated subnets (no route out of the VPC) and a security group
# without any rules. The execution role can only write logs, manage its ENIs and send traces.

data "aws_partition" "current" {}

locals {
  function_name = "${var.name_prefix}-runner-${var.language}"
  managed       = "arn:${data.aws_partition.current.partition}:iam::aws:policy"
}

resource "aws_security_group" "runner" {
  name                   = "${local.function_name}-sg"
  description            = "Runner ${var.language}: no inbound, no outbound"
  vpc_id                 = var.vpc_id
  revoke_rules_on_delete = true

  # No ingress or egress blocks: Terraform removes the default allow-all egress rule.

  tags = {
    Name = "${local.function_name}-sg"
  }
}

resource "aws_cloudwatch_log_group" "runner" {
  #checkov:skip=CKV_AWS_158:Runner logs contain no secrets or source code; the AWS owned key is enough.
  #checkov:skip=CKV_AWS_338:Short retention keeps cost near zero for a demo project.
  name              = "/aws/lambda/${local.function_name}"
  retention_in_days = var.log_retention_days
}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "runner" {
  name                 = "${local.function_name}-role"
  assume_role_policy   = data.aws_iam_policy_document.assume.json
  permissions_boundary = var.permissions_boundary_arn
}

resource "aws_iam_role_policy_attachment" "vpc_access" {
  role       = aws_iam_role.runner.name
  policy_arn = "${local.managed}/service-role/AWSLambdaVPCAccessExecutionRole"
}

resource "aws_iam_role_policy_attachment" "xray" {
  role       = aws_iam_role.runner.name
  policy_arn = "${local.managed}/AWSXRayDaemonWriteAccess"
}

resource "aws_lambda_function" "runner" {
  #checkov:skip=CKV_AWS_115:New accounts can have a concurrency quota of 10, which cannot be reserved; concurrency is limited by the worker instead.
  #checkov:skip=CKV_AWS_116:Invoked synchronously by the worker, which handles failures; a DLQ applies to async invokes only.
  #checkov:skip=CKV_AWS_272:Lambda code signing supports zip packages only; container images are signed with cosign in CI.
  #checkov:skip=CKV_AWS_173:The function has no environment variables.
  function_name = local.function_name
  description   = "CodeArena runner (${var.language})"
  # Every image change publishes a version; the worker calls the "live" alias.
  publish       = true
  role          = aws_iam_role.runner.arn
  package_type  = "Image"
  image_uri     = var.image_uri
  architectures = ["arm64"]
  memory_size   = var.memory_mb
  timeout       = var.timeout_seconds

  reserved_concurrent_executions = var.reserved_concurrency

  vpc_config {
    subnet_ids         = var.subnet_ids
    security_group_ids = [aws_security_group.runner.id]
  }

  tracing_config {
    mode = "Active"
  }

  logging_config {
    log_format = "Text"
    log_group  = aws_cloudwatch_log_group.runner.name
  }

  depends_on = [
    aws_iam_role_policy_attachment.vpc_access,
    aws_cloudwatch_log_group.runner,
  ]
}

# --- Alias the worker invokes, and the optional canary (ADR-0020) -------------------
# Without a canary Terraform points the alias at the new version. With one, CodeDeploy
# owns the alias: the pipeline starts a deployment that sends 10 % of the invocations
# to the new version for 5 minutes, after the BeforeAllowTraffic hook has judged an
# accepted solution with it, and rolls back when the alias reports function errors.

resource "aws_lambda_alias" "live" {
  count = var.canary == null ? 1 : 0

  name             = "live"
  function_name    = aws_lambda_function.runner.function_name
  function_version = aws_lambda_function.runner.version
}

resource "aws_lambda_alias" "live_canary" {
  count = var.canary == null ? 0 : 1

  name             = "live"
  function_name    = aws_lambda_function.runner.function_name
  function_version = aws_lambda_function.runner.version

  lifecycle {
    ignore_changes = [function_version, routing_config]
  }
}

locals {
  alias = one(concat(aws_lambda_alias.live[*], aws_lambda_alias.live_canary[*]))
}

resource "aws_cloudwatch_metric_alarm" "canary_errors" {
  count = var.canary == null ? 0 : 1

  alarm_name          = "${local.function_name}-live-errors"
  alarm_description   = "Runner crashed (function error) behind the live alias; CodeDeploy rolls the canary back."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  statistic           = "Sum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = aws_lambda_function.runner.function_name
    Resource     = "${aws_lambda_function.runner.function_name}:live"
  }
}

resource "aws_codedeploy_deployment_group" "runner" {
  count = var.canary == null ? 0 : 1

  app_name               = var.canary.app_name
  deployment_group_name  = local.function_name # the hook finds the function by this name
  service_role_arn       = var.canary.service_role_arn
  deployment_config_name = "CodeDeployDefault.LambdaCanary10Percent5Minutes"

  deployment_style {
    deployment_type   = "BLUE_GREEN"
    deployment_option = "WITH_TRAFFIC_CONTROL"
  }

  auto_rollback_configuration {
    enabled = true
    events  = ["DEPLOYMENT_FAILURE", "DEPLOYMENT_STOP_ON_ALARM"]
  }

  alarm_configuration {
    enabled = true
    alarms  = [aws_cloudwatch_metric_alarm.canary_errors[0].alarm_name]
  }
}
