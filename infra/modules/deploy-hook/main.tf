# Deployment hook Lambda (ADR-0020), lambdas/deploy-hook:
#   - ECS blue/green lifecycle hook of the API (POST_TEST_TRAFFIC_SHIFT): smoke test
#     through CloudFront with the test header, before production traffic moves;
#   - CodeDeploy BeforeAllowTraffic hook of the runner canary: the new runner version
#     judges an accepted solution before the alias sends it traffic.
# Outside the VPC: it needs CloudFront, SSM, CodeDeploy and the Lambda API only.

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}
data "aws_region" "current" {}

locals {
  function_name = "${var.name_prefix}-deploy-hook"
  partition     = data.aws_partition.current.partition
  managed       = "arn:${local.partition}:iam::aws:policy"
  arn_suffix    = "${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}"
}

data "archive_file" "hook" {
  type        = "zip"
  source_dir  = "${path.module}/../../../lambdas/deploy-hook/codearena_deploy_hook"
  output_path = "${path.root}/.terraform/tmp/${var.name_prefix}-deploy-hook.zip"
  excludes    = ["__pycache__", "__pycache__/*"]
}

resource "aws_cloudwatch_log_group" "hook" {
  #checkov:skip=CKV_AWS_158:The logs hold URLs, statuses and versions only; the AWS owned key is enough.
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

resource "aws_iam_role" "hook" {
  name                 = "${local.function_name}-role"
  assume_role_policy   = data.aws_iam_policy_document.assume.json
  permissions_boundary = var.permissions_boundary_arn
}

resource "aws_iam_role_policy_attachment" "logs" {
  role       = aws_iam_role.hook.name
  policy_arn = "${local.managed}/service-role/AWSLambdaBasicExecutionRole"
}

data "aws_iam_policy_document" "hook" {
  statement {
    sid       = "ReadSiteParameter"
    actions   = ["ssm:GetParameter"]
    resources = ["arn:${local.partition}:ssm:${local.arn_suffix}:parameter/${trimprefix(var.site_parameter_name, "/")}"]
  }

  statement {
    sid       = "JudgeNewRunnerVersions"
    actions   = ["lambda:InvokeFunction", "lambda:ListVersionsByFunction"]
    resources = flatten([for arn in values(var.runner_function_arns) : [arn, "${arn}:*"]])
  }

  dynamic "statement" {
    for_each = var.codedeploy_app_name == null ? [] : [var.codedeploy_app_name]
    content {
      sid       = "ReportToCodeDeploy"
      actions   = ["codedeploy:GetDeployment", "codedeploy:PutLifecycleEventHookExecutionStatus"]
      resources = ["arn:${local.partition}:codedeploy:${local.arn_suffix}:deploymentgroup:${statement.value}/*"]
    }
  }
}

resource "aws_iam_role_policy" "hook" {
  name   = "deploy-hook"
  role   = aws_iam_role.hook.id
  policy = data.aws_iam_policy_document.hook.json
}

resource "aws_lambda_function" "hook" {
  #checkov:skip=CKV_AWS_117:Outside the VPC on purpose: it calls the public CloudFront URL and AWS APIs.
  #checkov:skip=CKV_AWS_115:New accounts can have a concurrency quota of 10, which cannot be reserved; deployments call it a few times.
  #checkov:skip=CKV_AWS_116:Invoked synchronously by ECS and CodeDeploy, which handle failures (rollback).
  #checkov:skip=CKV_AWS_272:Code signing would need a signing profile per environment; the zip is built from reviewed code in the pipeline.
  #checkov:skip=CKV_AWS_173:The environment holds a parameter name, no secrets.
  function_name    = local.function_name
  description      = "CodeArena deployment smoke tests (ECS blue/green, runner canary)"
  role             = aws_iam_role.hook.arn
  runtime          = "python3.13"
  handler          = "handler.handler"
  architectures    = ["arm64"]
  filename         = data.archive_file.hook.output_path
  source_code_hash = data.archive_file.hook.output_base64sha256
  memory_size      = 128
  # A cold C++ runner compiles in a few seconds; the API calls time out after 10 seconds each.
  timeout = 120

  environment {
    variables = {
      SITE_PARAMETER = var.site_parameter_name
    }
  }

  tracing_config {
    mode = "Active"
  }

  logging_config {
    log_format = "Text"
    log_group  = aws_cloudwatch_log_group.hook.name
  }

  depends_on = [aws_iam_role_policy_attachment.logs, aws_cloudwatch_log_group.hook]
}

resource "aws_iam_role_policy_attachment" "xray" {
  role       = aws_iam_role.hook.name
  policy_arn = "${local.managed}/AWSXRayDaemonWriteAccess"
}
