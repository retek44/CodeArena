# IAM for the tasks. Every role carries the workload permissions boundary (ADR-0007).

data "aws_partition" "current" {}

data "aws_iam_policy_document" "ecs_tasks_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# Execution role: used by ECS itself to pull images, write logs and read the DB secret.
resource "aws_iam_role" "execution" {
  name                 = "${var.name_prefix}-ecs-execution"
  assume_role_policy   = data.aws_iam_policy_document.ecs_tasks_assume.json
  permissions_boundary = var.permissions_boundary_arn
}

resource "aws_iam_role_policy_attachment" "execution" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "execution_secrets" {
  statement {
    sid       = "ReadDatabasePassword"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [var.database.password_secret_arn]
  }
}

resource "aws_iam_role_policy" "execution_secrets" {
  name   = "database-secret"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.execution_secrets.json
}

# API task role: publish new submissions (outbox relay).
resource "aws_iam_role" "api" {
  name                 = "${var.name_prefix}-api-task"
  assume_role_policy   = data.aws_iam_policy_document.ecs_tasks_assume.json
  permissions_boundary = var.permissions_boundary_arn
}

data "aws_iam_policy_document" "api" {
  statement {
    sid       = "PublishSubmissions"
    actions   = ["sqs:SendMessage"]
    resources = [var.queue.arn]
  }

  dynamic "statement" {
    for_each = var.hint_function_arn == null ? [] : [var.hint_function_arn]
    content {
      sid       = "InvokeHint"
      actions   = ["lambda:InvokeFunction"]
      resources = [statement.value]
    }
  }

  dynamic "statement" {
    for_each = var.realtime == null ? [] : [var.realtime.table_arn]
    content {
      sid       = "IssueWebSocketTickets"
      actions   = ["dynamodb:PutItem"]
      resources = [statement.value]
    }
  }
}

resource "aws_iam_role_policy" "api" {
  name   = "api"
  role   = aws_iam_role.api.id
  policy = data.aws_iam_policy_document.api.json
}

# Worker task role: consume the queue and invoke the runner.
resource "aws_iam_role" "worker" {
  name                 = "${var.name_prefix}-worker-task"
  assume_role_policy   = data.aws_iam_policy_document.ecs_tasks_assume.json
  permissions_boundary = var.permissions_boundary_arn
}

data "aws_iam_policy_document" "worker" {
  statement {
    sid = "ConsumeSubmissions"
    actions = [
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:ChangeMessageVisibility",
      "sqs:GetQueueAttributes",
    ]
    resources = [var.queue.arn]
  }

  statement {
    sid       = "InvokeRunner"
    actions   = ["lambda:InvokeFunction"]
    resources = values(var.runner_function_arns)
  }

  dynamic "statement" {
    for_each = var.realtime == null ? [] : [var.realtime.bus_arn]
    content {
      sid       = "PublishJudgeEvents"
      actions   = ["events:PutEvents"]
      resources = [statement.value]
    }
  }
}

resource "aws_iam_role_policy" "worker" {
  name   = "worker"
  role   = aws_iam_role.worker.id
  policy = data.aws_iam_policy_document.worker.json
}

# One-off job tasks (migrate, seed): need no AWS access, only the database.
resource "aws_iam_role" "job" {
  name                 = "${var.name_prefix}-job-task"
  assume_role_policy   = data.aws_iam_policy_document.ecs_tasks_assume.json
  permissions_boundary = var.permissions_boundary_arn
}
