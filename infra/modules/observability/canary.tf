# CloudWatch Synthetics canary: sign in -> submit -> AC on a schedule (prod by default).
# Its user is created by the canary itself and gets a fresh random password on every
# run through the Cognito admin API, so no password is stored anywhere.

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

locals {
  canary_name     = substr(replace("${var.name_prefix}-journey", "codearena-", "ca-"), 0, 21)
  user_pool_arn   = "arn:${data.aws_partition.current.partition}:cognito-idp:${local.region}:${data.aws_caller_identity.current.account_id}:userpool/${var.cognito.user_pool_id}"
  canary_username = "synthetics-canary@example.invalid"
}

data "archive_file" "canary" {
  count = var.canary_enabled ? 1 : 0

  type        = "zip"
  source_dir  = "${path.module}/canary"
  output_path = "${path.root}/.terraform/tmp/${var.name_prefix}-canary.zip"
}

resource "aws_s3_bucket" "canary" {
  #checkov:skip=CKV_AWS_18:Canary artifacts (logs, results) expire after 7 days; access logs would add a second bucket.
  #checkov:skip=CKV_AWS_144:Artifacts of a monitoring check need no second region.
  #checkov:skip=CKV_AWS_145:SSE-S3 is enough for canary run results.
  #checkov:skip=CKV2_AWS_62:No consumer for event notifications.
  #checkov:skip=CKV_AWS_21:Run results are written once and expire; versioning adds nothing.
  count = var.canary_enabled ? 1 : 0

  bucket_prefix = "${var.name_prefix}-canary-"
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "canary" {
  count = var.canary_enabled ? 1 : 0

  bucket                  = aws_s3_bucket.canary[0].id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "canary" {
  count = var.canary_enabled ? 1 : 0

  bucket = aws_s3_bucket.canary[0].id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "canary" {
  count = var.canary_enabled ? 1 : 0

  bucket = aws_s3_bucket.canary[0].id
  rule {
    id     = "expire-runs"
    status = "Enabled"
    filter {}
    expiration {
      days = 7
    }
    abort_incomplete_multipart_upload {
      days_after_initiation = 1
    }
  }
}

data "aws_iam_policy_document" "canary_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "canary" {
  count = var.canary_enabled ? 1 : 0

  name                 = "${var.name_prefix}-canary"
  assume_role_policy   = data.aws_iam_policy_document.canary_assume.json
  permissions_boundary = var.permissions_boundary_arn
}

data "aws_iam_policy_document" "canary" {
  count = var.canary_enabled ? 1 : 0

  statement {
    sid       = "Artifacts"
    actions   = ["s3:PutObject", "s3:GetBucketLocation"]
    resources = [aws_s3_bucket.canary[0].arn, "${aws_s3_bucket.canary[0].arn}/*"]
  }

  statement {
    sid       = "ListBuckets"
    actions   = ["s3:ListAllMyBuckets"]
    resources = ["*"]
  }

  statement {
    sid       = "Logs"
    actions   = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["arn:${data.aws_partition.current.partition}:logs:${local.region}:${data.aws_caller_identity.current.account_id}:log-group:/aws/lambda/cwsyn-*"]
  }

  statement {
    sid       = "Metrics"
    actions   = ["cloudwatch:PutMetricData"]
    resources = ["*"]
    condition {
      test     = "StringEquals"
      variable = "cloudwatch:namespace"
      values   = ["CloudWatchSynthetics"]
    }
  }

  statement {
    sid       = "CanaryUser"
    actions   = ["cognito-idp:AdminCreateUser", "cognito-idp:AdminSetUserPassword", "cognito-idp:AdminInitiateAuth"]
    resources = [local.user_pool_arn]
  }
}

resource "aws_iam_role_policy" "canary" {
  count = var.canary_enabled ? 1 : 0

  name   = "canary"
  role   = aws_iam_role.canary[0].id
  policy = data.aws_iam_policy_document.canary[0].json
}

resource "aws_synthetics_canary" "journey" {
  count = var.canary_enabled ? 1 : 0

  name                 = local.canary_name
  artifact_s3_location = "s3://${aws_s3_bucket.canary[0].id}/"
  execution_role_arn   = aws_iam_role.canary[0].arn
  runtime_version      = var.canary_runtime
  handler              = "canary.handler"
  zip_file             = data.archive_file.canary[0].output_path
  start_canary         = true
  delete_lambda        = true

  success_retention_period = 2
  failure_retention_period = 7

  schedule {
    expression = var.canary_schedule
  }

  run_config {
    timeout_in_seconds = 300
    memory_in_mb       = 960
    environment_variables = {
      SITE_URL        = var.site_url
      USER_POOL_ID    = var.cognito.user_pool_id
      CLIENT_ID       = var.cognito.client_id
      CANARY_USERNAME = local.canary_username
    }
  }

  depends_on = [aws_iam_role_policy.canary]
}

resource "aws_cloudwatch_metric_alarm" "canary" {
  count = var.canary_enabled ? 1 : 0

  alarm_name          = "${var.name_prefix}-canary-failed"
  alarm_description   = "The sign in -> submit -> AC journey failed twice in a row."
  namespace           = "CloudWatchSynthetics"
  metric_name         = "SuccessPercent"
  dimensions          = { CanaryName = aws_synthetics_canary.journey[0].name }
  statistic           = "Average"
  period              = 1800
  evaluation_periods  = 2
  threshold           = 100
  comparison_operator = "LessThanThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [var.alerts_topic_arn]
  ok_actions          = [var.alerts_topic_arn]
}
