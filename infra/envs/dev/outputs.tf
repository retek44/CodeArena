output "site_url" {
  description = "Public URL of the SPA."
  value       = module.app.site_url
}

output "cognito" {
  description = "Values for the SPA build."
  value       = module.app.cognito
}

output "deploy" {
  description = "Values the pipeline needs after apply."
  value       = module.app.deploy
}
