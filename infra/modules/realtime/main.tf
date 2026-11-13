# Live updates (ADR-0015, contracts/asyncapi.yaml):
#
#   worker --PutEvents--> EventBridge bus --rule--> push Lambda --> API Gateway WebSocket --> browser
#
# The browser connects with a single-use ticket from POST /api/ws/ticket (the API writes
# it into the table, the $connect Lambda consumes it). Everything here is pay per use:
# an idle environment costs nothing beyond the CloudWatch log storage.

data "aws_partition" "current" {}
data "aws_region" "current" {}

locals {
  partition  = data.aws_partition.current.partition
  region     = data.aws_region.current.region
  managed    = "arn:${local.partition}:iam::aws:policy"
  stage      = "live"
  management = "https://${aws_apigatewayv2_api.ws.id}.execute-api.${local.region}.amazonaws.com/${local.stage}"
}

# --- State: tickets, connections, subscriptions ----------------------------------------

resource "aws_dynamodb_table" "ws" {
  #checkov:skip=CKV_AWS_28:Tickets and connections live at most two hours; a backup has nothing to restore.
  #checkov:skip=CKV_AWS_119:Encrypted with the AWS owned key; the items hold random tickets, connection IDs and user IDs only.
  #checkov:skip=CKV2_AWS_16:On-demand capacity scales by itself; there is nothing to auto scale.
  name         = "${var.name_prefix}-ws"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "pk"
  range_key    = "sk"

  attribute {
    name = "pk"
    type = "S"
  }

  attribute {
    name = "sk"
    type = "S"
  }

  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }
}

# --- Events -------------------------------------------------------------------------------

resource "aws_cloudwatch_event_bus" "this" {
  #checkov:skip=CKV_AWS_385:Events carry submission IDs and statuses; the AWS owned key is enough.
  name = var.name_prefix
}

resource "aws_cloudwatch_event_rule" "submission_updated" {
  name           = "${var.name_prefix}-submission-updated"
  description    = "Submission and contest changes -> WebSocket push"
  event_bus_name = aws_cloudwatch_event_bus.this.name

  event_pattern = jsonencode({
    source      = ["codearena.judge"]
    detail-type = ["Submission updated", "Leaderboard updated", "Contest phase changed"]
  })
}

resource "aws_cloudwatch_event_target" "push" {
  rule           = aws_cloudwatch_event_rule.submission_updated.name
  event_bus_name = aws_cloudwatch_event_bus.this.name
  arn            = aws_lambda_function.this["push"].arn

  retry_policy {
    # A late update is worthless: the SPA polls anyway.
    maximum_event_age_in_seconds = 60
    maximum_retry_attempts       = 2
  }
}

resource "aws_lambda_permission" "events" {
  statement_id  = "AllowEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.this["push"].function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.submission_updated.arn
}

# --- Functions ------------------------------------------------------------------------------

locals {
  functions = {
    # $connect, $disconnect, subscribe and $default; handler.route dispatches.
    ws = {
      handler = "handler.route"
      actions = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:DeleteItem"]
    }
    push = {
      handler = "handler.push"
      actions = ["dynamodb:Query", "dynamodb:DeleteItem"]
    }
  }
}

data "archive_file" "ws" {
  type        = "zip"
  source_dir  = "${path.module}/../../../lambdas/ws/codearena_ws"
  output_path = "${path.root}/.terraform/tmp/${var.name_prefix}-ws.zip"
  excludes    = ["__pycache__", "__pycache__/*"]
}

resource "aws_cloudwatch_log_group" "function" {
  #checkov:skip=CKV_AWS_158:The logs hold counts and statuses only; the AWS owned key is enough.
  #checkov:skip=CKV_AWS_338:Short retention keeps cost near zero for a demo project.
  for_each = local.functions

  name              = "/aws/lambda/${var.name_prefix}-${each.key}"
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

resource "aws_iam_role" "function" {
  for_each = local.functions

  name                 = "${var.name_prefix}-${each.key}-role"
  assume_role_policy   = data.aws_iam_policy_document.assume.json
  permissions_boundary = var.permissions_boundary_arn
}

resource "aws_iam_role_policy_attachment" "logs" {
  for_each = local.functions

  role       = aws_iam_role.function[each.key].name
  policy_arn = "${local.managed}/service-role/AWSLambdaBasicExecutionRole"
}

data "aws_iam_policy_document" "function" {
  for_each = local.functions

  statement {
    sid       = "Table"
    actions   = each.value.actions
    resources = [aws_dynamodb_table.ws.arn]
  }

  statement {
    sid       = "PostToConnections"
    actions   = ["execute-api:ManageConnections"]
    resources = ["${aws_apigatewayv2_api.ws.execution_arn}/${local.stage}/POST/@connections/*"]
  }
}

resource "aws_iam_role_policy" "function" {
  for_each = local.functions

  name   = "ws"
  role   = aws_iam_role.function[each.key].id
  policy = data.aws_iam_policy_document.function[each.key].json
}

resource "aws_lambda_function" "this" {
  #checkov:skip=CKV_AWS_117:Outside the VPC on purpose: it needs only DynamoDB and the API Gateway management API.
  #checkov:skip=CKV_AWS_115:New accounts can have a concurrency quota of 10, which cannot be reserved; the stage throttles instead.
  #checkov:skip=CKV_AWS_116:Invoked synchronously by API Gateway, or by EventBridge with its own retry policy.
  #checkov:skip=CKV_AWS_272:Code signing would need a signing profile per environment; the zip is built from reviewed code in the pipeline.
  #checkov:skip=CKV_AWS_173:The environment holds a table name and an endpoint, no secrets.
  #checkov:skip=CKV_AWS_50:X-Ray adds little to two short functions; the judge traces live in the worker.
  for_each = local.functions

  function_name    = "${var.name_prefix}-${each.key}"
  description      = "CodeArena WebSocket (${each.key})"
  role             = aws_iam_role.function[each.key].arn
  runtime          = "python3.13"
  handler          = each.value.handler
  architectures    = ["arm64"]
  filename         = data.archive_file.ws.output_path
  source_code_hash = data.archive_file.ws.output_base64sha256
  memory_size      = 128
  timeout          = 10

  environment {
    variables = {
      TABLE_NAME  = aws_dynamodb_table.ws.name
      WS_ENDPOINT = local.management
    }
  }

  logging_config {
    log_format = "Text"
    log_group  = aws_cloudwatch_log_group.function[each.key].name
  }

  depends_on = [aws_iam_role_policy_attachment.logs, aws_cloudwatch_log_group.function]
}

# --- WebSocket API ----------------------------------------------------------------------------

resource "aws_apigatewayv2_api" "ws" {
  name                       = "${var.name_prefix}-ws"
  protocol_type              = "WEBSOCKET"
  route_selection_expression = "$request.body.action"
}

resource "aws_apigatewayv2_integration" "ws" {
  api_id                    = aws_apigatewayv2_api.ws.id
  integration_type          = "AWS_PROXY"
  integration_uri           = aws_lambda_function.this["ws"].invoke_arn
  content_handling_strategy = "CONVERT_TO_TEXT"
}

resource "aws_apigatewayv2_route" "ws" {
  #checkov:skip=CKV_AWS_309:$connect authenticates with the single-use ticket inside the Lambda (ADR-0015); WebSocket APIs offer no JWT authorizer.
  for_each = toset(["$connect", "$disconnect", "$default", "subscribe"])

  api_id    = aws_apigatewayv2_api.ws.id
  route_key = each.key
  target    = "integrations/${aws_apigatewayv2_integration.ws.id}"
}

resource "aws_lambda_permission" "api" {
  statement_id  = "AllowApiGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.this["ws"].function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.ws.execution_arn}/*/*"
}

resource "aws_cloudwatch_log_group" "access" {
  #checkov:skip=CKV_AWS_158:Access logs hold request metadata only; the AWS owned key is enough.
  #checkov:skip=CKV_AWS_338:Short retention keeps cost near zero for a demo project.
  name              = "/aws/apigateway/${var.name_prefix}-ws"
  retention_in_days = var.log_retention_days
}

resource "aws_apigatewayv2_stage" "live" {
  #checkov:skip=CKV2_AWS_51:Browsers connect directly; client certificates (mTLS) do not apply to a public WebSocket API.
  api_id      = aws_apigatewayv2_api.ws.id
  name        = local.stage
  auto_deploy = true

  default_route_settings {
    throttling_burst_limit = 50
    throttling_rate_limit  = 20
  }

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.access.arn
    format = jsonencode({
      requestId    = "$context.requestId"
      routeKey     = "$context.routeKey"
      status       = "$context.status"
      connectionId = "$context.connectionId"
      error        = "$context.error.message"
    })
  }
}
