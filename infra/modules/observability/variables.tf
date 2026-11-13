variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-dev."
  type        = string
}

variable "permissions_boundary_arn" {
  description = "Workload permissions boundary (created by infra/bootstrap)."
  type        = string
}

variable "alerts_topic_arn" {
  description = "SNS topic for alarms."
  type        = string
}

variable "site_url" {
  description = "Public URL of the environment (CloudFront)."
  type        = string
}

variable "cognito" {
  description = "User pool and SPA client the canary signs in with."
  type = object({
    user_pool_id = string
    client_id    = string
  })
}

variable "dashboard" {
  description = "Names and dimensions the dashboard shows."
  type = object({
    alb_arn_suffix          = string
    target_group_arn_suffix = string
    cluster_name            = string
    api_service_name        = string
    worker_service_name     = string
    queue_name              = string
    dlq_name                = string
    db_identifier           = string
    runner_function_names   = map(string)
    worker_log_group        = string
    api_log_group           = string
  })
}

variable "canary_enabled" {
  description = "Run the Synthetics canary (about 0.0012 USD per run). Prod only by default."
  type        = bool
  default     = false
}

variable "canary_schedule" {
  description = "Canary schedule expression."
  type        = string
  default     = "rate(30 minutes)"
}

variable "canary_runtime" {
  description = "Synthetics runtime (aws synthetics describe-runtime-versions)."
  type        = string
  default     = "syn-nodejs-5.3"
}
