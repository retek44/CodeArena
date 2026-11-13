output "address" {
  description = "Hostname (DB_HOST)."
  value       = aws_db_instance.this.address
}

output "port" {
  description = "Port."
  value       = aws_db_instance.this.port
}

output "database" {
  description = "Database name (DB_NAME)."
  value       = aws_db_instance.this.db_name
}

output "username" {
  description = "Master user name (DB_USER)."
  value       = aws_db_instance.this.username
}

output "password_secret_arn" {
  description = "Secrets Manager secret with the generated password (JSON key: password)."
  value       = aws_db_instance.this.master_user_secret[0].secret_arn
}
