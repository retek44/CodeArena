# Contest lifecycle task (ADR-0016): `contest start|freeze|end|finalize <id>`, started
# by the Step Functions state machine of infra/modules/contests at the contest's times.
# Same network as the one-off jobs: public subnets with a public IP, no inbound traffic.

data "aws_caller_identity" "current" {}

resource "aws_iam_role" "contest" {
  count = var.contests == null ? 0 : 1

  name                 = "${var.name_prefix}-contest-task"
  assume_role_policy   = data.aws_iam_policy_document.ecs_tasks_assume.json
  permissions_boundary = var.permissions_boundary_arn
}

data "aws_iam_policy_document" "contest" {
  count = var.contests == null ? 0 : 1

  dynamic "statement" {
    for_each = var.realtime == null ? [] : [var.realtime.bus_arn]
    content {
      sid       = "PublishPhaseEvents"
      actions   = ["events:PutEvents"]
      resources = [statement.value]
    }
  }

  statement {
    sid       = "FindParticipantEmails"
    actions   = ["cognito-idp:ListUsers"]
    resources = [var.contests.cognito_user_pool]
  }

  dynamic "statement" {
    for_each = var.contests.email_from == "" ? [] : [var.contests.email_from]
    content {
      sid     = "SendResultEmails"
      actions = ["ses:SendEmail"]
      # Identities of the verified sender (address or its domain) in this account.
      resources = ["arn:${data.aws_partition.current.partition}:ses:${local.region}:${data.aws_caller_identity.current.account_id}:identity/*"]
      condition {
        test     = "StringEquals"
        variable = "ses:FromAddress"
        values   = [statement.value]
      }
    }
  }
}

resource "aws_iam_role_policy" "contest" {
  count = var.contests == null ? 0 : 1

  name   = "contest"
  role   = aws_iam_role.contest[0].id
  policy = data.aws_iam_policy_document.contest[0].json
}

resource "aws_ecs_task_definition" "contest" {
  count = var.contests == null ? 0 : 1

  family                   = "${var.name_prefix}-contest"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.contest[0].arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([{
    name                   = "contest"
    image                  = var.contests.image
    essential              = true
    readonlyRootFilesystem = true
    user                   = "65532:65532"
    environment = concat(local.database_env, local.valkey_env, [
      { name = "COGNITO_USER_POOL_ID", value = var.contests.cognito_user_pool_id },
      { name = "CONTEST_EMAIL_FROM", value = var.contests.email_from },
      ], var.realtime == null ? [] : [
      { name = "EVENT_BUS_NAME", value = var.realtime.bus },
    ])
    secrets = local.database_secret
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.service["contest"].name
        awslogs-region        = local.region
        awslogs-stream-prefix = "contest"
      }
    }
  }])
}

# The seed schedules the lifecycle of new contests: a one-time schedule per contest.
data "aws_iam_policy_document" "seed_scheduling" {
  count = var.contests == null ? 0 : 1

  statement {
    sid       = "CreateContestSchedules"
    actions   = ["scheduler:CreateSchedule"]
    resources = ["arn:${data.aws_partition.current.partition}:scheduler:${local.region}:${data.aws_caller_identity.current.account_id}:schedule/${var.contests.scheduler_group}/*"]
  }

  statement {
    sid       = "PassSchedulerRole"
    actions   = ["iam:PassRole"]
    resources = [var.contests.scheduler_role_arn]
    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["scheduler.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "seed_scheduling" {
  count = var.contests == null ? 0 : 1

  name   = "seed-scheduling"
  role   = aws_iam_role.job.id
  policy = data.aws_iam_policy_document.seed_scheduling[0].json
}
