output "repository_urls" {
  description = "ECR repository URL per image."
  value       = { for name, repo in module.ecr : name => repo.repository_url }
}
