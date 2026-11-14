variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-prod."
  type        = string
}

variable "permissions_boundary_arn" {
  description = "Workload permissions boundary (created by infra/bootstrap)."
  type        = string
}

variable "site_parameter_name" {
  description = "SecureString parameter with the site URL and the test header value (written by the app module)."
  type        = string
}

variable "runner_function_arns" {
  description = "Runner functions (unqualified ARNs) whose new versions the CodeDeploy hook judges."
  type        = map(string)
}

variable "codedeploy_app_name" {
  description = "CodeDeploy application of the runner canary; null when the environment has no canary."
  type        = string
  default     = null
}

variable "log_retention_days" {
  description = "CloudWatch log retention."
  type        = number
  default     = 14
}
