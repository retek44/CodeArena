output "queue_url" {
  description = "Queue URL (SUBMISSIONS_QUEUE_URL for the API and the worker)."
  value       = aws_sqs_queue.this.url
}

output "queue_arn" {
  description = "Queue ARN (IAM: sqs:SendMessage for the API, Receive/Delete/ChangeMessageVisibility for the worker)."
  value       = aws_sqs_queue.this.arn
}

output "dlq_arn" {
  description = "Dead-letter queue ARN (alarm on visible messages in phase 7)."
  value       = aws_sqs_queue.dlq.arn
}

output "queue_name" {
  description = "Queue name (CloudWatch metric dimension for worker autoscaling)."
  value       = aws_sqs_queue.this.name
}

output "visibility_timeout_seconds" {
  description = "Visibility timeout; the worker lease must match it."
  value       = aws_sqs_queue.this.visibility_timeout_seconds
}

output "dlq_alarm_name" {
  description = "CloudWatch alarm on dead letters."
  value       = aws_cloudwatch_metric_alarm.dlq_not_empty.alarm_name
}

output "dlq_name" {
  description = "Dead-letter queue name (dashboard)."
  value       = aws_sqs_queue.dlq.name
}
