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

output "migrate_task_definition_arn" {
  description = "Migration task definition (aws ecs run-task in the pipeline)."
  value       = aws_ecs_task_definition.migrate.arn
}

output "task_subnet_ids" {
  description = "Subnets for run-task."
  value       = var.public_subnet_ids
}

output "security_group_ids" {
  description = "Security groups per component (database ingress, run-task)."
  value = {
    api     = aws_security_group.api.id
    worker  = aws_security_group.worker.id
    migrate = aws_security_group.migrate.id
  }
}
