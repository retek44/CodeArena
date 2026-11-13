# AWS FIS experiment: stop the running worker tasks while submissions are judged.
# The judge must still finish every submission exactly once (leases, redelivery,
# sweeper). The template costs nothing; a run is billed per action minute (cents).
# The non-functional workflow (.github/workflows/nonfunctional.yml) starts it under load.

data "aws_iam_policy_document" "fis_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["fis.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "fis" {
  name                 = "${var.name_prefix}-fis"
  assume_role_policy   = data.aws_iam_policy_document.fis_assume.json
  permissions_boundary = var.permissions_boundary_arn
}

data "aws_iam_policy_document" "fis" {
  #checkov:skip=CKV_AWS_356:ecs:ListTasks has no resource-level permissions; it is limited to this cluster by the ecs:cluster condition.
  statement {
    sid       = "StopWorkerTasks"
    actions   = ["ecs:StopTask", "ecs:DescribeTasks"]
    resources = ["arn:${data.aws_partition.current.partition}:ecs:*:*:task/${aws_ecs_cluster.this.name}/*"]
  }

  statement {
    sid       = "DescribeCluster"
    actions   = ["ecs:DescribeClusters"]
    resources = [aws_ecs_cluster.this.arn]
  }

  statement {
    sid       = "FindWorkerTasks"
    actions   = ["ecs:ListTasks"]
    resources = ["*"]
    condition {
      test     = "ArnEquals"
      variable = "ecs:cluster"
      values   = [aws_ecs_cluster.this.arn]
    }
  }
}

resource "aws_iam_role_policy" "fis" {
  name   = "stop-worker-tasks"
  role   = aws_iam_role.fis.id
  policy = data.aws_iam_policy_document.fis.json
}

resource "aws_fis_experiment_template" "stop_worker_tasks" {
  description = "Stop all running worker tasks mid-judge; every submission must still finish exactly once."
  role_arn    = aws_iam_role.fis.arn

  action {
    name        = "stop-worker-tasks"
    action_id   = "aws:ecs:stop-task"
    description = "Stop the worker tasks (autoscaling starts new ones)."

    target {
      key   = "Tasks"
      value = "worker-tasks"
    }
  }

  target {
    name           = "worker-tasks"
    resource_type  = "aws:ecs:task"
    selection_mode = "ALL"

    parameters = {
      cluster = aws_ecs_cluster.this.name
      service = aws_ecs_service.worker.name
    }
  }

  stop_condition {
    source = "none"
  }

  tags = {
    Name = "${var.name_prefix}-stop-worker-tasks"
  }
}
