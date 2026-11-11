output "function_name" {
  description = "Runner function name."
  value       = aws_lambda_function.runner.function_name
}

output "function_arn" {
  description = "Runner function ARN (the worker needs lambda:InvokeFunction on it)."
  value       = aws_lambda_function.runner.arn
}

output "security_group_id" {
  description = "Security group of the runner (no rules)."
  value       = aws_security_group.runner.id
}
