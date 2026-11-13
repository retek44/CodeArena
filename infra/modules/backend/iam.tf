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
    resources = [var.runner_function_arn]
  }
}

resource "aws_iam_role_policy" "worker" {
  name   = "worker"
  role   = aws_iam_role.worker.id
  policy = data.aws_iam_policy_document.worker.json
}

# Migration task role: needs no AWS access (only the database).
resource "aws_iam_role" "migrate" {
  name                 = "${var.name_prefix}-migrate-task"
  assume_role_policy   = data.aws_iam_policy_document.ecs_tasks_assume.json
  permissions_boundary = var.permissions_boundary_arn
}
