output "plan_role_arn" {
  description = "ARN of the read-only plan role."
  value       = aws_iam_role.plan.arn
}

output "deploy_role_arns" {
  description = "Deploy role ARN per environment."
  value       = { for env, role in aws_iam_role.deploy : env => role.arn }
}

output "workload_boundary_arn" {
  description = "Permissions boundary that every CI-created IAM role must use."
  value       = aws_iam_policy.boundary.arn
}
