output "name_prefix" {
  description = "Prefix of every resource name in this environment."
  value       = local.name_prefix
}

output "site_url" {
  description = "Public URL of the SPA (null without image_tag)."
  value       = local.site_url
}

output "submissions_queue_url" {
  description = "SUBMISSIONS_QUEUE_URL."
  value       = module.submissions_queue.queue_url
}

output "cognito" {
  description = "Values for the SPA build (VITE_COGNITO_*) and the API."
  value = {
    authority = module.cognito.issuer
    client_id = module.cognito.client_id
    domain    = module.cognito.login_domain
  }
}

output "deploy" {
  description = "Values the pipeline needs after terraform apply (null without image_tag)."
  value = local.deploy ? {
    cluster                     = module.backend[0].cluster_name
    api_service                 = module.backend[0].api_service_name
    worker_service              = module.backend[0].worker_service_name
    migrate_task_definition_arn = module.backend[0].migrate_task_definition_arn
    task_subnet_ids             = module.backend[0].task_subnet_ids
    migrate_security_group_id   = module.backend[0].security_group_ids["migrate"]
    web_bucket                  = module.web[0].bucket
    distribution_id             = module.web[0].distribution_id
    runner_function_name        = module.runner_python[0].function_name
  } : null
}
