# API and worker on ECS Fargate (arm64), behind an ALB that only CloudFront can reach.

data "aws_region" "current" {}

locals {
  region = data.aws_region.current.region

  database_env = [
    { name = "DB_HOST", value = var.database.host },
    { name = "DB_NAME", value = var.database.name },
    { name = "DB_USER", value = var.database.user },
  ]
  database_secret = [
    { name = "DB_PASSWORD", valueFrom = "${var.database.password_secret_arn}:password::" },
  ]
}

resource "aws_ecs_cluster" "this" {
  name = var.name_prefix

  setting {
    name  = "containerInsights"
    value = "enhanced"
  }
}

resource "aws_ecs_cluster_capacity_providers" "this" {
  cluster_name       = aws_ecs_cluster.this.name
  capacity_providers = ["FARGATE", "FARGATE_SPOT"]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }
}

resource "aws_cloudwatch_log_group" "service" {
  #checkov:skip=CKV_AWS_158:Application logs hold no secrets; the AWS owned key is enough.
  #checkov:skip=CKV_AWS_338:Short retention keeps cost near zero for a demo project.
  for_each = toset(["api", "worker", "migrate"])

  name              = "/ecs/${var.name_prefix}/${each.key}"
  retention_in_days = var.log_retention_days
}

# --- Security groups -------------------------------------------------------------

resource "aws_security_group" "api" {
  name                   = "${var.name_prefix}-api"
  description            = "API tasks: HTTP from the ALB only"
  vpc_id                 = var.vpc_id
  revoke_rules_on_delete = true
}

resource "aws_vpc_security_group_ingress_rule" "api_from_alb" {
  security_group_id            = aws_security_group.api.id
  referenced_security_group_id = aws_security_group.alb.id
  ip_protocol                  = "tcp"
  from_port                    = 8080
  to_port                      = 8080
  description                  = "HTTP from the ALB"
}

resource "aws_security_group" "worker" {
  name                   = "${var.name_prefix}-worker"
  description            = "Worker tasks: no inbound traffic"
  vpc_id                 = var.vpc_id
  revoke_rules_on_delete = true
}

resource "aws_security_group" "migrate" {
  #checkov:skip=CKV2_AWS_5:Attached at runtime: the pipeline runs the migration task with this security group.
  name                   = "${var.name_prefix}-migrate"
  description            = "Migration task: no inbound traffic"
  vpc_id                 = var.vpc_id
  revoke_rules_on_delete = true
}

# Outbound HTTPS (AWS APIs, Cognito JWKS, ECR) and PostgreSQL inside the VPC.
resource "aws_vpc_security_group_egress_rule" "https" {
  for_each = {
    api     = aws_security_group.api.id
    worker  = aws_security_group.worker.id
    migrate = aws_security_group.migrate.id
  }

  security_group_id = each.value
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = "0.0.0.0/0"
  description       = "HTTPS to AWS APIs and Cognito"
}

data "aws_vpc" "this" {
  id = var.vpc_id
}

resource "aws_vpc_security_group_egress_rule" "postgres" {
  for_each = {
    api     = aws_security_group.api.id
    worker  = aws_security_group.worker.id
    migrate = aws_security_group.migrate.id
  }

  security_group_id = each.value
  ip_protocol       = "tcp"
  from_port         = 5432
  to_port           = 5432
  cidr_ipv4         = data.aws_vpc.this.cidr_block
  description       = "PostgreSQL inside the VPC"
}

# --- API service -----------------------------------------------------------------

resource "aws_ecs_task_definition" "api" {
  family                   = "${var.name_prefix}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.api.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([{
    name                   = "api"
    image                  = var.images.api
    essential              = true
    readonlyRootFilesystem = true
    user                   = "65532:65532"
    portMappings           = [{ containerPort = 8080, protocol = "tcp" }]
    environment = concat(local.database_env, [
      { name = "PORT", value = "8080" },
      { name = "AUTH_MODE", value = "cognito" },
      { name = "COGNITO_ISSUER", value = var.cognito.issuer },
      { name = "COGNITO_CLIENT_ID", value = var.cognito.client_id },
      { name = "SUBMISSIONS_QUEUE_URL", value = var.queue.url },
    ])
    secrets = local.database_secret
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.service["api"].name
        awslogs-region        = local.region
        awslogs-stream-prefix = "api"
      }
    }
  }])
}

resource "aws_ecs_service" "api" {
  #checkov:skip=CKV_AWS_333:Public IPs instead of a NAT Gateway (ADR-0005); inbound is limited to the ALB security group.
  name                   = "${var.name_prefix}-api"
  cluster                = aws_ecs_cluster.this.id
  task_definition        = aws_ecs_task_definition.api.arn
  desired_count          = var.api_desired_count
  launch_type            = "FARGATE"
  enable_execute_command = false
  propagate_tags         = "SERVICE"

  network_configuration {
    subnets          = var.public_subnet_ids
    security_groups  = [aws_security_group.api.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "api"
    container_port   = 8080
  }

  # A deployment that does not become healthy, or trips the 5xx alarm, rolls back by itself.
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  alarms {
    alarm_names = [aws_cloudwatch_metric_alarm.api_5xx.alarm_name]
    enable      = true
    rollback    = true
  }

  depends_on = [aws_lb_listener_rule.api]
}

resource "aws_cloudwatch_metric_alarm" "api_5xx" {
  alarm_name          = "${var.name_prefix}-api-5xx"
  alarm_description   = "API returns server errors; ECS rolls back a deployment when this fires."
  namespace           = "AWS/ApplicationELB"
  metric_name         = "HTTPCode_Target_5XX_Count"
  statistic           = "Sum"
  period              = 60
  evaluation_periods  = 2
  threshold           = 5
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"

  dimensions = {
    LoadBalancer = aws_lb.this.arn_suffix
    TargetGroup  = aws_lb_target_group.api.arn_suffix
  }
}

# --- Worker service --------------------------------------------------------------

resource "aws_ecs_task_definition" "worker" {
  family                   = "${var.name_prefix}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.worker.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([{
    name                   = "worker"
    image                  = var.images.worker
    essential              = true
    readonlyRootFilesystem = true
    user                   = "65532:65532"
    # ECS sends SIGTERM and waits this long: the worker finishes the submission in hand.
    stopTimeout = 120
    environment = concat(local.database_env, [
      { name = "SUBMISSIONS_QUEUE_URL", value = var.queue.url },
      { name = "RUNNER_MODE", value = "lambda" },
      { name = "RUNNER_FUNCTION_PYTHON", value = var.runner_function_arn },
      { name = "WORKER_LEASE", value = "${var.queue.visibility_timeout_seconds}s" },
      { name = "WORKER_CONCURRENCY", value = "4" },
    ])
    secrets = local.database_secret
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.service["worker"].name
        awslogs-region        = local.region
        awslogs-stream-prefix = "worker"
      }
    }
  }])
}

resource "aws_ecs_service" "worker" {
  #checkov:skip=CKV_AWS_333:Public IPs instead of a NAT Gateway (ADR-0005); the worker security group allows no inbound traffic.
  name                   = "${var.name_prefix}-worker"
  cluster                = aws_ecs_cluster.this.id
  task_definition        = aws_ecs_task_definition.worker.arn
  desired_count          = 0
  enable_execute_command = false
  propagate_tags         = "SERVICE"

  # Interrupted Spot tasks are safe: their leases expire and messages are redelivered.
  capacity_provider_strategy {
    capacity_provider = "FARGATE_SPOT"
    weight            = 1
  }

  network_configuration {
    subnets          = var.public_subnet_ids
    security_groups  = [aws_security_group.worker.id]
    assign_public_ip = true
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  # Autoscaling owns the task count.
  lifecycle {
    ignore_changes = [desired_count]
  }

  depends_on = [aws_ecs_cluster_capacity_providers.this]
}

# --- Migration task (run by the pipeline before each deploy) ---------------------

resource "aws_ecs_task_definition" "migrate" {
  family                   = "${var.name_prefix}-migrate"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.migrate.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  # Read-only root file system; Atlas gets a writable /tmp on task ephemeral storage.
  volume {
    name = "tmp"
  }

  container_definitions = jsonencode([{
    name                   = "migrate"
    image                  = var.images.migrate
    essential              = true
    readonlyRootFilesystem = true
    mountPoints            = [{ sourceVolume = "tmp", containerPath = "/tmp", readOnly = false }]
    environment            = local.database_env
    secrets                = local.database_secret
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.service["migrate"].name
        awslogs-region        = local.region
        awslogs-stream-prefix = "migrate"
      }
    }
  }])
}
