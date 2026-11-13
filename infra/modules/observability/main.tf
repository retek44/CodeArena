# One CloudWatch dashboard per environment: the edge, the API, the judge pipeline and
# the database on one page. Built-in AWS metrics and Logs Insights queries over the
# JSON logs only, so it adds no custom-metric cost (the first 3 dashboards are free).

data "aws_region" "current" {}

locals {
  region = data.aws_region.current.region
  d      = var.dashboard

  runner_metrics = flatten([
    for lang, fn in local.d.runner_function_names : [
      ["AWS/Lambda", "Duration", "FunctionName", fn, { stat = "p95", label = "${lang} p95 duration" }],
      ["AWS/Lambda", "Errors", "FunctionName", fn, { stat = "Sum", label = "${lang} errors", yAxis = "right" }],
      ["AWS/Lambda", "ConcurrentExecutions", "FunctionName", fn, { stat = "Maximum", label = "${lang} concurrency", yAxis = "right" }],
    ]
  ])

  widgets = [
    {
      type = "text", x = 0, y = 0, width = 24, height = 2,
      properties = {
        markdown = "## ${var.name_prefix}\n[Site](${var.site_url}) · API → outbox → SQS → worker (Fargate) → runner Lambdas · alarms → SNS `${var.name_prefix}-alerts`"
      }
    },
    {
      type = "metric", x = 0, y = 2, width = 12, height = 6,
      properties = {
        title  = "API: requests, 5xx, p95 latency (ALB)"
        region = local.region
        view   = "timeSeries"
        metrics = [
          ["AWS/ApplicationELB", "RequestCount", "LoadBalancer", local.d.alb_arn_suffix, "TargetGroup", local.d.target_group_arn_suffix, { stat = "Sum" }],
          ["AWS/ApplicationELB", "HTTPCode_Target_5XX_Count", "LoadBalancer", local.d.alb_arn_suffix, "TargetGroup", local.d.target_group_arn_suffix, { stat = "Sum" }],
          ["AWS/ApplicationELB", "TargetResponseTime", "LoadBalancer", local.d.alb_arn_suffix, "TargetGroup", local.d.target_group_arn_suffix, { stat = "p95", yAxis = "right" }],
        ]
      }
    },
    {
      type = "metric", x = 12, y = 2, width = 12, height = 6,
      properties = {
        title  = "Queue: backlog, age of oldest message, dead letters"
        region = local.region
        view   = "timeSeries"
        metrics = [
          ["AWS/SQS", "ApproximateNumberOfMessagesVisible", "QueueName", local.d.queue_name, { stat = "Maximum" }],
          ["AWS/SQS", "ApproximateNumberOfMessagesNotVisible", "QueueName", local.d.queue_name, { stat = "Maximum", label = "in flight" }],
          ["AWS/SQS", "ApproximateAgeOfOldestMessage", "QueueName", local.d.queue_name, { stat = "Maximum", yAxis = "right" }],
          ["AWS/SQS", "ApproximateNumberOfMessagesVisible", "QueueName", local.d.dlq_name, { stat = "Maximum", label = "DLQ" }],
        ]
      }
    },
    {
      type = "metric", x = 0, y = 8, width = 12, height = 6,
      properties = {
        title  = "ECS: running tasks and CPU"
        region = local.region
        view   = "timeSeries"
        metrics = [
          ["ECS/ContainerInsights", "RunningTaskCount", "ClusterName", local.d.cluster_name, "ServiceName", local.d.api_service_name, { stat = "Maximum", label = "api tasks" }],
          ["ECS/ContainerInsights", "RunningTaskCount", "ClusterName", local.d.cluster_name, "ServiceName", local.d.worker_service_name, { stat = "Maximum", label = "worker tasks" }],
          ["AWS/ECS", "CPUUtilization", "ClusterName", local.d.cluster_name, "ServiceName", local.d.api_service_name, { stat = "Average", yAxis = "right", label = "api CPU %" }],
          ["AWS/ECS", "CPUUtilization", "ClusterName", local.d.cluster_name, "ServiceName", local.d.worker_service_name, { stat = "Average", yAxis = "right", label = "worker CPU %" }],
        ]
      }
    },
    {
      type = "metric", x = 12, y = 8, width = 12, height = 6,
      properties = {
        title   = "Runner Lambdas"
        region  = local.region
        view    = "timeSeries"
        metrics = local.runner_metrics
      }
    },
    {
      type = "log", x = 0, y = 14, width = 12, height = 6,
      properties = {
        title  = "Verdicts by language (worker logs)"
        region = local.region
        view   = "bar"
        query  = "SOURCE '${local.d.worker_log_group}' | filter msg = \"submission judged\" | stats count() as submissions by verdict, language"
      }
    },
    {
      type = "log", x = 12, y = 14, width = 12, height = 6,
      properties = {
        title  = "Judge latency in the worker, ms (p50 / p95 / max per 5 min)"
        region = local.region
        view   = "timeSeries"
        query  = "SOURCE '${local.d.worker_log_group}' | filter msg = \"processed submission\" and outcome = \"judged\" | stats pct(duration_ms, 50) as p50, pct(duration_ms, 95) as p95, max(duration_ms) as max by bin(5m)"
      }
    },
    {
      type = "log", x = 0, y = 20, width = 12, height = 6,
      properties = {
        title  = "Recovered submissions (sweeper) and outbox warnings (API logs)"
        region = local.region
        view   = "table"
        query  = "SOURCE '${local.d.api_log_group}' | filter msg like /stuck submission|publish outbox message/ | stats count() by msg"
      }
    },
    {
      type = "metric", x = 12, y = 20, width = 12, height = 6,
      properties = {
        title  = "Database"
        region = local.region
        view   = "timeSeries"
        metrics = [
          ["AWS/RDS", "CPUUtilization", "DBInstanceIdentifier", local.d.db_identifier, { stat = "Average" }],
          ["AWS/RDS", "DatabaseConnections", "DBInstanceIdentifier", local.d.db_identifier, { stat = "Maximum", yAxis = "right" }],
          ["AWS/RDS", "FreeStorageSpace", "DBInstanceIdentifier", local.d.db_identifier, { stat = "Minimum", yAxis = "right" }],
        ]
      }
    },
  ]

  canary_widget = var.canary_enabled ? [{
    type = "metric", x = 0, y = 26, width = 24, height = 4,
    properties = {
      title  = "Synthetics canary: sign in -> submit -> AC"
      region = local.region
      view   = "timeSeries"
      metrics = [
        ["CloudWatchSynthetics", "SuccessPercent", "CanaryName", aws_synthetics_canary.journey[0].name, { stat = "Average" }],
        ["CloudWatchSynthetics", "Duration", "CanaryName", aws_synthetics_canary.journey[0].name, { stat = "Average", yAxis = "right" }],
      ]
    }
  }] : []
}

resource "aws_cloudwatch_dashboard" "this" {
  dashboard_name = var.name_prefix
  dashboard_body = jsonencode({ widgets = concat(local.widgets, local.canary_widget) })
}
