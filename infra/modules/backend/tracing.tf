# OpenTelemetry tracing (ADR-0018). The API and the worker send spans over OTLP/HTTP
# to an ADOT collector sidecar on localhost; the collector exports them to X-Ray. The
# runner and hint Lambdas use Lambda active tracing and join the same trace through
# the X-Amzn-Trace-Id header the SDK middleware sends.

locals {
  # Read by backend/internal/telemetry; without them the services record no spans.
  otel_env = var.tracing.enabled ? [
    { name = "OTEL_EXPORTER_OTLP_ENDPOINT", value = "http://localhost:4318" },
    { name = "OTEL_TRACES_SAMPLER", value = "parentbased_traceidratio" },
    { name = "OTEL_TRACES_SAMPLER_ARG", value = tostring(var.tracing.sample_ratio) },
    { name = "OTEL_RESOURCE_ATTRIBUTES", value = "deployment.environment.name=${var.name_prefix}" },
  ] : []

  adot_config = yamlencode({
    receivers = {
      otlp = { protocols = { http = { endpoint = "127.0.0.1:4318" } } }
    }
    processors = {
      batch = {}
    }
    exporters = {
      awsxray = {
        region = local.region
        # Searchable in X-Ray as annotations (annotation.codearena_submission_id = "...").
        indexed_attributes = ["codearena.submission_id"]
      }
    }
    service = {
      pipelines = {
        traces = { receivers = ["otlp"], processors = ["batch"], exporters = ["awsxray"] }
      }
    }
  })

  # Not essential: a failing collector costs traces, never the service.
  adot_container = {
    for service in ["api", "worker"] : service => var.tracing.enabled ? [{
      name                   = "adot-collector"
      image                  = var.adot_image
      essential              = false
      readonlyRootFilesystem = true
      memoryReservation      = 64
      environment            = [{ name = "AOT_CONFIG_CONTENT", value = local.adot_config }]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.service[service].name
          awslogs-region        = local.region
          awslogs-stream-prefix = "adot"
        }
      }
    }] : []
  }
}

# PutTraceSegments and the sampling-rule reads the collector needs.
resource "aws_iam_role_policy_attachment" "xray" {
  for_each = var.tracing.enabled ? { api = aws_iam_role.api.name, worker = aws_iam_role.worker.name } : {}

  role       = each.value
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AWSXRayDaemonWriteAccess"
}
