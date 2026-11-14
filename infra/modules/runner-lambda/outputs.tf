output "function_name" {
  description = "Runner function name."
  value       = aws_lambda_function.runner.function_name
}

output "function_arn" {
  description = "Runner function ARN, unqualified."
  value       = aws_lambda_function.runner.arn
}

output "alias_arn" {
  description = "ARN of the live alias: the worker invokes it and needs lambda:InvokeFunction on it."
  value       = local.alias.arn
}

output "version" {
  description = "Version Terraform published last (the target of a canary deployment)."
  value       = aws_lambda_function.runner.version
}

output "deployment_group" {
  description = "CodeDeploy deployment group of the canary (null without canary)."
  value       = one(aws_codedeploy_deployment_group.runner[*].deployment_group_name)
}

output "security_group_id" {
  description = "Security group of the runner (no rules)."
  value       = aws_security_group.runner.id
}
