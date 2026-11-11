output "name" {
  description = "Repository name."
  value       = aws_ecr_repository.this.name
}

output "arn" {
  description = "Repository ARN."
  value       = aws_ecr_repository.this.arn
}

output "repository_url" {
  description = "Registry URL without tag."
  value       = aws_ecr_repository.this.repository_url
}
