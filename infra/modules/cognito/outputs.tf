output "user_pool_id" {
  description = "User pool ID."
  value       = aws_cognito_user_pool.this.id
}

output "client_id" {
  description = "SPA app client ID. The API accepts only access tokens issued to this client."
  value       = aws_cognito_user_pool_client.spa.id
}

output "issuer" {
  description = "Token issuer; the API reads the signing keys from <issuer>/.well-known/jwks.json."
  value       = "https://${aws_cognito_user_pool.this.endpoint}"
}

output "login_domain" {
  description = "Managed login domain for the SPA."
  value       = "${aws_cognito_user_pool_domain.this.domain}.auth.${data.aws_region.current.region}.amazoncognito.com"
}

data "aws_region" "current" {}

output "user_pool_arn" {
  description = "User pool ARN (IAM policies)."
  value       = aws_cognito_user_pool.this.arn
}
