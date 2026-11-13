# Contests (ADR-0016):
#   - Valkey (ElastiCache Serverless) caches the leaderboards as sorted sets;
#   - the seed creates a one-time EventBridge Scheduler schedule per contest that
#     starts the Step Functions state machine at the start time;
#   - the state machine runs the lifecycle task (backend `contest` command) at the
#     start, the freeze, the end and two minutes later (results, e-mails).
# The phases follow the clock in the API and the worker, so a late or failed step
# never changes who can submit or what the leaderboard shows.

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}
data "aws_region" "current" {}

data "aws_vpc" "this" {
  id = var.vpc_id
}

locals {
  partition = data.aws_partition.current.partition
  region    = data.aws_region.current.region
  account   = data.aws_caller_identity.current.account_id
}

# --- Valkey ---------------------------------------------------------------------------

resource "aws_security_group" "valkey" {
  name                   = "${var.name_prefix}-valkey"
  description            = "Valkey leaderboard cache: inbound from the VPC only"
  vpc_id                 = var.vpc_id
  revoke_rules_on_delete = true

  tags = {
    Name = "${var.name_prefix}-valkey"
  }
}

resource "aws_vpc_security_group_ingress_rule" "valkey" {
  security_group_id = aws_security_group.valkey.id
  ip_protocol       = "tcp"
  from_port         = 6379
  to_port           = 6380
  cidr_ipv4         = data.aws_vpc.this.cidr_block
  description       = "Valkey from the API, worker and job tasks"
}

resource "aws_elasticache_serverless_cache" "leaderboard" {
  #checkov:skip=CKV_AWS_191:Encrypted at rest with the AWS owned key; the cache holds public leaderboard rows only.
  engine               = "valkey"
  name                 = "${var.name_prefix}-leaderboard"
  description          = "Contest leaderboards (sorted sets); Postgres is the source of truth"
  major_engine_version = "8"
  subnet_ids           = var.subnet_ids
  security_group_ids   = [aws_security_group.valkey.id]

  # Bounds the cost: the boards are tiny.
  cache_usage_limits {
    data_storage {
      maximum = 1
      unit    = "GB"
    }
    ecpu_per_second {
      maximum = 5000
    }
  }
}

# --- Lifecycle state machine -------------------------------------------------------------

locals {
  steps = ["start", "freeze", "end", "finalize"]
  run_step = { for step in local.steps : step => {
    Type     = "Task"
    Resource = "arn:${local.partition}:states:::ecs:runTask.sync"
    Parameters = {
      Cluster        = var.task.cluster_arn
      TaskDefinition = var.task.task_definition_arn
      LaunchType     = "FARGATE"
      NetworkConfiguration = {
        AwsvpcConfiguration = {
          Subnets        = var.task.subnet_ids
          SecurityGroups = [var.task.security_group_id]
          AssignPublicIp = "ENABLED"
        }
      }
      Overrides = {
        ContainerOverrides = [{
          Name        = "contest"
          "Command.$" = "States.Array('${step}', States.Format('{}', $.contest_id))"
        }]
      }
    }
    # Keep the input (contest ID and times) for the next states.
    ResultPath = null
    Retry = [{
      ErrorEquals     = ["States.TaskFailed", "ECS.AmazonECSException", "States.Timeout"]
      IntervalSeconds = 30
      MaxAttempts     = 3
      BackoffRate     = 2
    }]
    TimeoutSeconds = 900
  } }
}

resource "aws_cloudwatch_log_group" "machine" {
  #checkov:skip=CKV_AWS_158:Execution history holds contest IDs and times; the AWS owned key is enough.
  #checkov:skip=CKV_AWS_338:Short retention keeps cost near zero for a demo project.
  name              = "/aws/states/${var.state_machine_name}"
  retention_in_days = var.log_retention_days
}

data "aws_iam_policy_document" "states_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["states.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [local.account]
    }
  }
}

resource "aws_iam_role" "machine" {
  name                 = "${var.state_machine_name}-sfn"
  assume_role_policy   = data.aws_iam_policy_document.states_assume.json
  permissions_boundary = var.permissions_boundary_arn
}

data "aws_iam_policy_document" "machine" {
  #checkov:skip=CKV_AWS_356:ecs:DescribeTasks/StopTask are limited to the cluster by condition; log delivery actions do not support resource ARNs.
  #checkov:skip=CKV_AWS_111:Log delivery actions do not support resource ARNs (AWS documented requirement for Step Functions logging).
  statement {
    sid       = "RunLifecycleTask"
    actions   = ["ecs:RunTask"]
    resources = ["arn:${local.partition}:ecs:${local.region}:${local.account}:task-definition/${var.task.task_family}:*"]
    condition {
      test     = "ArnEquals"
      variable = "ecs:cluster"
      values   = [var.task.cluster_arn]
    }
  }

  statement {
    sid       = "WatchLifecycleTask"
    actions   = ["ecs:StopTask", "ecs:DescribeTasks"]
    resources = ["*"]
    condition {
      test     = "ArnEquals"
      variable = "ecs:cluster"
      values   = [var.task.cluster_arn]
    }
  }

  statement {
    sid       = "PassTaskRoles"
    actions   = ["iam:PassRole"]
    resources = var.task.pass_role_arns
    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["ecs-tasks.amazonaws.com"]
    }
  }

  # runTask.sync waits for the task through a managed EventBridge rule.
  statement {
    sid       = "SyncWithEcs"
    actions   = ["events:PutTargets", "events:PutRule", "events:DescribeRule"]
    resources = ["arn:${local.partition}:events:${local.region}:${local.account}:rule/StepFunctionsGetEventsForECSTaskRule"]
  }

  statement {
    sid = "DeliverLogs"
    actions = [
      "logs:CreateLogDelivery", "logs:GetLogDelivery", "logs:UpdateLogDelivery", "logs:DeleteLogDelivery",
      "logs:ListLogDeliveries", "logs:PutResourcePolicy", "logs:DescribeResourcePolicies", "logs:DescribeLogGroups",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "machine" {
  name   = "contest-lifecycle"
  role   = aws_iam_role.machine.id
  policy = data.aws_iam_policy_document.machine.json
}

resource "aws_iam_role_policy_attachment" "machine_xray" {
  role       = aws_iam_role.machine.name
  policy_arn = "arn:${local.partition}:iam::aws:policy/AWSXRayDaemonWriteAccess"
}

resource "aws_sfn_state_machine" "lifecycle" {
  name     = var.state_machine_name
  role_arn = aws_iam_role.machine.arn

  definition = jsonencode({
    Comment = "Contest lifecycle: start, freeze, end, finalize (ADR-0016). Input: contest_id, slug, freeze_at, ends_at."
    StartAt = "Start"
    States = {
      Start          = merge(local.run_step["start"], { Next = "WaitForFreeze" })
      WaitForFreeze  = { Type = "Wait", TimestampPath = "$.freeze_at", Next = "Freeze" }
      Freeze         = merge(local.run_step["freeze"], { Next = "WaitForEnd" })
      WaitForEnd     = { Type = "Wait", TimestampPath = "$.ends_at", Next = "End" }
      End            = merge(local.run_step["end"], { Next = "WaitForJudging" })
      WaitForJudging = { Type = "Wait", Seconds = 120, Next = "Finalize" }
      Finalize       = merge(local.run_step["finalize"], { End = true })
    }
  })

  # Full execution history: the input holds only the contest ID, slug and times.
  logging_configuration {
    log_destination        = "${aws_cloudwatch_log_group.machine.arn}:*"
    include_execution_data = true
    level                  = "ALL"
  }

  tracing_configuration {
    enabled = true
  }

  depends_on = [aws_iam_role_policy.machine]
}

# --- Scheduler ---------------------------------------------------------------------------

resource "aws_scheduler_schedule_group" "contests" {
  name = var.schedule_group
}

data "aws_iam_policy_document" "scheduler_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [local.account]
    }
  }
}

resource "aws_iam_role" "scheduler" {
  name                 = var.scheduler_role_name
  assume_role_policy   = data.aws_iam_policy_document.scheduler_assume.json
  permissions_boundary = var.permissions_boundary_arn
}

data "aws_iam_policy_document" "scheduler" {
  statement {
    sid       = "StartLifecycle"
    actions   = ["states:StartExecution"]
    resources = [aws_sfn_state_machine.lifecycle.arn]
  }
}

resource "aws_iam_role_policy" "scheduler" {
  name   = "start-contest-lifecycle"
  role   = aws_iam_role.scheduler.id
  policy = data.aws_iam_policy_document.scheduler.json
}
