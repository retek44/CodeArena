# AI hint (ADR-0014): a Python Lambda (lambdas/hint) calls a Claude model on Amazon
# Bedrock through a guardrail. The API invokes it synchronously; it stores one hint per
# submission and limits hints per user, so the cost stays at a few cents a day.
#
# The function runs outside the VPC: it reaches only the Bedrock API with its own role,
# and the isolated subnets have no route to Bedrock (an interface endpoint costs
# ~8 USD a month per AZ). It handles no database credentials and no hidden test data.

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}
data "aws_region" "current" {}

locals {
  function_name = "${var.name_prefix}-hint"
  partition     = data.aws_partition.current.partition
  region        = data.aws_region.current.region
  managed       = "arn:${local.partition}:iam::aws:policy"

  # Cross-region inference profile (eu.*): the call may run in any EU region of the
  # profile, so the role needs the profile and the foundation model in those regions.
  profile_arn = "arn:${local.partition}:bedrock:${local.region}:${data.aws_caller_identity.current.account_id}:inference-profile/${var.model_id}"
  base_model  = replace(var.model_id, "/^(eu|us|apac|global)\\./", "")
}

# --- Guardrail -------------------------------------------------------------------------

resource "aws_bedrock_guardrail" "hint" {
  name        = local.function_name
  description = "CodeArena hints: no full solutions, no prompt attacks, no harmful content."

  blocked_input_messaging   = "This request cannot get a hint."
  blocked_outputs_messaging = "No hint is available for this submission."

  content_policy_config {
    # MEDIUM, not HIGH: source code is full of words such as kill, execute or abort.
    dynamic "filters_config" {
      for_each = ["HATE", "INSULTS", "SEXUAL", "VIOLENCE", "MISCONDUCT"]
      content {
        type            = filters_config.value
        input_strength  = "MEDIUM"
        output_strength = "MEDIUM"
      }
    }

    # Applies to the guardContent block only: the user's source and its comments.
    filters_config {
      type            = "PROMPT_ATTACK"
      input_strength  = "HIGH"
      output_strength = "NONE"
    }
  }

  topic_policy_config {
    topics_config {
      name       = "full-solution"
      type       = "DENY"
      definition = "Giving a complete or corrected program, full source code, or a step-by-step algorithm that solves the whole programming problem."
      examples = [
        "Here is the corrected code that passes all tests.",
        "Write the full solution for me.",
        "Replace your program with this one.",
      ]
    }
  }

  sensitive_information_policy_config {
    # The hint must not echo credentials that a user pasted into the source.
    pii_entities_config {
      type   = "AWS_ACCESS_KEY"
      action = "ANONYMIZE"
    }
    pii_entities_config {
      type   = "AWS_SECRET_KEY"
      action = "ANONYMIZE"
    }
    pii_entities_config {
      type   = "PASSWORD"
      action = "ANONYMIZE"
    }
  }
}

# The Lambda uses a numbered version, never DRAFT. A change of the guardrail creates a new version.
resource "aws_bedrock_guardrail_version" "hint" {
  guardrail_arn = aws_bedrock_guardrail.hint.guardrail_arn
  description   = "Managed by Terraform"

  lifecycle {
    replace_triggered_by = [aws_bedrock_guardrail.hint]
  }
}

# --- Function ----------------------------------------------------------------------------

data "archive_file" "hint" {
  type        = "zip"
  source_dir  = "${path.module}/../../../lambdas/hint/codearena_hint"
  output_path = "${path.root}/.terraform/tmp/${local.function_name}.zip"
  excludes    = ["__pycache__", "__pycache__/*"]
}

resource "aws_cloudwatch_log_group" "hint" {
  #checkov:skip=CKV_AWS_158:The logs hold verdicts and token counts only (no source, no hint text); the AWS owned key is enough.
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

resource "aws_iam_role" "hint" {
  name                 = "${local.function_name}-role"
  assume_role_policy   = data.aws_iam_policy_document.assume.json
  permissions_boundary = var.permissions_boundary_arn
}

resource "aws_iam_role_policy_attachment" "logs" {
  role       = aws_iam_role.hint.name
  policy_arn = "${local.managed}/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy_attachment" "xray" {
  role       = aws_iam_role.hint.name
  policy_arn = "${local.managed}/AWSXRayDaemonWriteAccess"
}

data "aws_iam_policy_document" "bedrock" {
  statement {
    sid       = "InvokeThroughProfile"
    actions   = ["bedrock:InvokeModel"]
    resources = [local.profile_arn]
  }

  # The profile forwards to the model in one of its regions; allowed only through the profile.
  statement {
    sid       = "InvokeModelInProfileRegions"
    actions   = ["bedrock:InvokeModel"]
    resources = ["arn:${local.partition}:bedrock:*::foundation-model/${local.base_model}"]

    condition {
      test     = "StringEquals"
      variable = "bedrock:InferenceProfileArn"
      values   = [local.profile_arn]
    }
  }

  statement {
    sid       = "ApplyGuardrail"
    actions   = ["bedrock:ApplyGuardrail"]
    resources = [aws_bedrock_guardrail.hint.guardrail_arn]
  }
}

resource "aws_iam_role_policy" "bedrock" {
  name   = "bedrock"
  role   = aws_iam_role.hint.id
  policy = data.aws_iam_policy_document.bedrock.json
}

resource "aws_lambda_function" "hint" {
  #checkov:skip=CKV_AWS_117:Outside the VPC on purpose: it calls only the Bedrock API (see the header comment).
  #checkov:skip=CKV_AWS_115:New accounts can have a concurrency quota of 10, which cannot be reserved; the API rate limit caps calls.
  #checkov:skip=CKV_AWS_116:Invoked synchronously by the API, which handles failures; a DLQ applies to async invokes only.
  #checkov:skip=CKV_AWS_272:Code signing would need a signing profile per environment; the zip is built from reviewed code in the pipeline.
  #checkov:skip=CKV_AWS_173:The environment holds a model ID and a guardrail ID, no secrets.
  function_name    = local.function_name
  description      = "CodeArena AI hint (Bedrock with guardrail)"
  role             = aws_iam_role.hint.arn
  runtime          = "python3.13"
  handler          = "handler.handler"
  architectures    = ["arm64"]
  filename         = data.archive_file.hint.output_path
  source_code_hash = data.archive_file.hint.output_base64sha256
  memory_size      = 256
  # Same as the API's hint timeout (backend/internal/httpapi/hints.go); the model needs 2-6 s.
  timeout = 20

  environment {
    variables = {
      MODEL_ID          = var.model_id
      GUARDRAIL_ID      = aws_bedrock_guardrail.hint.guardrail_id
      GUARDRAIL_VERSION = aws_bedrock_guardrail_version.hint.version
    }
  }

  tracing_config {
    mode = "Active"
  }

  logging_config {
    log_format = "Text"
    log_group  = aws_cloudwatch_log_group.hint.name
  }

  depends_on = [
    aws_iam_role_policy_attachment.logs,
    aws_cloudwatch_log_group.hint,
  ]
}
