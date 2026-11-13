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
    cluster               = module.backend[0].cluster_name
    api_service           = module.backend[0].api_service_name
    worker_service        = module.backend[0].worker_service_name
    job_task_definitions  = module.backend[0].job_task_definition_arns
    task_subnet_ids       = module.backend[0].task_subnet_ids
    job_security_group_id = module.backend[0].security_group_ids["job"]
    alb_url               = "http://${module.backend[0].alb_dns_name}"
    web_bucket            = module.web[0].bucket
    distribution_id       = module.web[0].distribution_id
    runner_function_name  = module.runner_python[0].function_name
  } : null
}

# Separate from deploy: a targeted apply only writes an output whose dependencies are
# all in the targeted graph. This one needs only the network, the cluster, the job
# tasks and their security group, so migrate and seed can read it before the services
# exist. Keep the -target list in deploy-environment.yml in step with it.
output "jobs" {
  description = "Values for the one-off migrate and seed ECS tasks (null without image_tag)."
  value = local.deploy ? {
    cluster               = module.backend[0].cluster_name
    job_task_definitions  = module.backend[0].job_task_definition_arns
    task_subnet_ids       = module.backend[0].task_subnet_ids
    job_security_group_id = module.backend[0].job_security_group_id
  } : null
}
