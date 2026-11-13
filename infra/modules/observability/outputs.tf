output "dashboard_name" {
  description = "CloudWatch dashboard of the environment."
  value       = aws_cloudwatch_dashboard.this.dashboard_name
}

output "canary_name" {
  description = "Synthetics canary (null when disabled)."
  value       = var.canary_enabled ? aws_synthetics_canary.journey[0].name : null
}
