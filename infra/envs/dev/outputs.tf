output "name_prefix" {
  description = "Prefix for every resource name in this environment."
  value       = local.name_prefix
}

output "vpc_id" {
  description = "VPC ID."
  value       = module.network.vpc_id
}

output "submissions_queue_url" {
  description = "SUBMISSIONS_QUEUE_URL for the API and the worker."
  value       = module.submissions_queue.queue_url
}

output "cognito_issuer" {
  description = "API setting COGNITO_ISSUER."
  value       = module.cognito.issuer
}

output "cognito_client_id" {
  description = "API setting COGNITO_CLIENT_ID and the SPA client ID."
  value       = module.cognito.client_id
}

output "cognito_login_domain" {
  description = "Managed login domain for the SPA."
  value       = module.cognito.login_domain
}

output "runner_python_function_name" {
  description = "Python runner function, if deployed."
  value       = one(module.runner_python[*].function_name)
}
