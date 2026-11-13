# Worker scaling on the queue: 0 tasks when the queue is empty, more tasks when
# messages wait. Step scaling on ApproximateNumberOfMessagesVisible.

resource "aws_appautoscaling_target" "worker" {
  service_namespace  = "ecs"
  resource_id        = "service/${aws_ecs_cluster.this.name}/${aws_ecs_service.worker.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  min_capacity       = 0
  max_capacity       = var.worker_max_count
}

resource "aws_appautoscaling_policy" "worker_out" {
  name               = "${var.name_prefix}-worker-out"
  service_namespace  = aws_appautoscaling_target.worker.service_namespace
  resource_id        = aws_appautoscaling_target.worker.resource_id
  scalable_dimension = aws_appautoscaling_target.worker.scalable_dimension
  policy_type        = "StepScaling"

  step_scaling_policy_configuration {
    adjustment_type         = "ExactCapacity"
    cooldown                = 60
    metric_aggregation_type = "Maximum"

    # 1-9 waiting messages: 1 task; 10-49: 2 tasks; 50+: max.
    step_adjustment {
      metric_interval_lower_bound = 0
      metric_interval_upper_bound = 9
      scaling_adjustment          = 1
    }
    step_adjustment {
      metric_interval_lower_bound = 9
      metric_interval_upper_bound = 49
      scaling_adjustment          = min(2, var.worker_max_count)
    }
    step_adjustment {
      metric_interval_lower_bound = 49
      scaling_adjustment          = var.worker_max_count
    }
  }
}

resource "aws_cloudwatch_metric_alarm" "queue_has_work" {
  alarm_name          = "${var.name_prefix}-queue-has-work"
  alarm_description   = "Messages wait for the judge: scale workers out."
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  dimensions          = { QueueName = var.queue.name }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_appautoscaling_policy.worker_out.arn]
}

resource "aws_appautoscaling_policy" "worker_in" {
  name               = "${var.name_prefix}-worker-in"
  service_namespace  = aws_appautoscaling_target.worker.service_namespace
  resource_id        = aws_appautoscaling_target.worker.resource_id
  scalable_dimension = aws_appautoscaling_target.worker.scalable_dimension
  policy_type        = "StepScaling"

  step_scaling_policy_configuration {
    adjustment_type         = "ExactCapacity"
    cooldown                = 300
    metric_aggregation_type = "Maximum"

    step_adjustment {
      metric_interval_upper_bound = 0
      scaling_adjustment          = 0
    }
  }
}

# Empty for 10 minutes (nothing visible and nothing in flight): scale to zero.
resource "aws_cloudwatch_metric_alarm" "queue_idle" {
  alarm_name          = "${var.name_prefix}-queue-idle"
  alarm_description   = "No work for 10 minutes: scale workers to zero."
  evaluation_periods  = 10
  threshold           = 0
  comparison_operator = "LessThanOrEqualToThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [aws_appautoscaling_policy.worker_in.arn]

  metric_query {
    id          = "work"
    expression  = "visible + inflight"
    label       = "Messages waiting or being judged"
    return_data = true
  }

  metric_query {
    id = "visible"

    metric {
      namespace   = "AWS/SQS"
      metric_name = "ApproximateNumberOfMessagesVisible"
      dimensions  = { QueueName = var.queue.name }
      stat        = "Maximum"
      period      = 60
    }
  }

  metric_query {
    id = "inflight"

    metric {
      namespace   = "AWS/SQS"
      metric_name = "ApproximateNumberOfMessagesNotVisible"
      dimensions  = { QueueName = var.queue.name }
      stat        = "Maximum"
      period      = 60
    }
  }
}
