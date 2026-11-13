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
