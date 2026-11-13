output "alb_dns_name" {
  description = "ALB host name (CloudFront origin for /api/*)."
  value       = aws_lb.this.dns_name
}

output "origin_verify_header" {
  description = "Secret header value CloudFront sends to the ALB."
  value       = random_password.origin_verify.result
  sensitive   = true
}

output "cluster_name" {
  description = "ECS cluster."
  value       = aws_ecs_cluster.this.name
}

output "api_service_name" {
  description = "API service (the pipeline waits for it to become stable)."
  value       = aws_ecs_service.api.name
}

output "worker_service_name" {
  description = "Worker service."
  value       = aws_ecs_service.worker.name
}

output "job_task_definition_arns" {
  description = "One-off job task definitions by name (aws ecs run-task in the pipeline)."
  value       = { for name, task in aws_ecs_task_definition.job : name => task.arn }
}

output "task_subnet_ids" {
  description = "Subnets for run-task."
  value       = var.public_subnet_ids
}

output "security_group_ids" {
  description = "Security groups per component (database ingress, run-task)."
  value = {
    api    = aws_security_group.api.id
    worker = aws_security_group.worker.id
    job    = aws_security_group.job.id
  }
}

output "contest" {
  description = "Contest lifecycle task (Step Functions runs it); null without contests."
  value = var.contests == null ? null : {
    task_definition_arn = aws_ecs_task_definition.contest[0].arn
    task_family         = aws_ecs_task_definition.contest[0].family
    pass_role_arns      = [aws_iam_role.execution.arn, aws_iam_role.contest[0].arn]
    cluster_arn         = aws_ecs_cluster.this.arn
    subnet_ids          = var.public_subnet_ids
    security_group_id   = aws_security_group.job.id
  }
}

output "job_security_group_id" {
  description = "Security group of the one-off job tasks (run-task)."
  value       = aws_security_group.job.id
}

output "fis_stop_worker_template_id" {
  description = "FIS experiment template that stops the worker tasks (non-functional workflow)."
  value       = aws_fis_experiment_template.stop_worker_tasks.id
}

output "worker_autoscaling_resource_id" {
  description = "Application Auto Scaling resource of the worker service (scaling checks)."
  value       = aws_appautoscaling_target.worker.resource_id
}

output "alb_arn_suffix" {
  description = "ALB ARN suffix (CloudWatch dimension LoadBalancer)."
  value       = aws_lb.this.arn_suffix
}

output "api_target_group_arn_suffix" {
  description = "API target group ARN suffix (CloudWatch dimension TargetGroup)."
  value       = aws_lb_target_group.api.arn_suffix
}

output "log_group_names" {
  description = "CloudWatch log groups by component (api, worker, migrate, seed)."
  value       = { for name, group in aws_cloudwatch_log_group.service : name => group.name }
}
