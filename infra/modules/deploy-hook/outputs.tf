output "function_arn" {
  description = "Hook function (ECS lifecycle hook target, CodeDeploy BeforeAllowTraffic hook)."
  value       = aws_lambda_function.hook.arn
}

output "function_name" {
  description = "Hook function name (the AppSpec of a runner canary names it)."
  value       = aws_lambda_function.hook.function_name
}
