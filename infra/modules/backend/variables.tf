variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-dev."
  type        = string
}

variable "vpc_id" {
  description = "VPC ID."
  type        = string
}

variable "public_subnet_ids" {
  description = "Public subnets for the ALB and the tasks (tasks get a public IP; ADR-0005)."
  type        = list(string)
}

variable "permissions_boundary_arn" {
  description = "Workload permissions boundary for every IAM role (ADR-0007)."
  type        = string
}

variable "images" {
  description = "Container image URIs with tag."
  type = object({
    api     = string
    worker  = string
    migrate = string
    seed    = string
  })
}

variable "database" {
  description = "Connection details; the password comes from the RDS-managed secret."
  type = object({
    host                = string
    name                = string
    user                = string
    password_secret_arn = string
  })
}

variable "cognito" {
  description = "Token issuer and SPA client ID accepted by the API."
  type = object({
    issuer    = string
    client_id = string
  })
}

variable "queue" {
  description = "Submission queue."
  type = object({
    url                        = string
    arn                        = string
    name                       = string
    visibility_timeout_seconds = number
  })
}

variable "runner_function_arns" {
  description = "Runner Lambda per language (python, cpp) the worker invokes."
  type        = map(string)

  validation {
    condition     = contains(keys(var.runner_function_arns), "python")
    error_message = "The Python runner is required: custom checkers run on it."
  }
}

variable "hint_function_arn" {
  description = "Hint Lambda the API invokes; null disables hints (the endpoint answers 503)."
  type        = string
  default     = null
}

variable "realtime" {
  description = "WebSocket API and event bus (infra/modules/realtime); null disables live updates."
  type = object({
    ws_url    = string
    table     = string
    table_arn = string
    bus       = string
    bus_arn   = string
  })
  default = null
}

variable "contests" {
  description = <<-EOT
    Contest lifecycle (infra/modules/contests): the task image and the names the seed
    schedules with. The state machine and scheduler
    role ARNs are derived from their names, so this module does not depend on them.
    null disables the contest lifecycle; leaderboards then come from Postgres.
  EOT
  type = object({
    image                = string
    email_from           = string
    cognito_user_pool_id = string
    cognito_user_pool    = string
    state_machine_arn    = string
    scheduler_group      = string
    scheduler_role_arn   = string
  })
  default = null
}

variable "valkey_url" {
  description = "Leaderboard cache (rediss://...); null means leaderboards come from Postgres only."
  type        = string
  default     = null
}

variable "api_desired_count" {
  description = "API tasks."
  type        = number
  default     = 1
}

variable "worker_max_count" {
  description = "Upper bound for queue-driven worker scaling (it scales to zero when the queue is empty)."
  type        = number
  default     = 3
}

variable "log_retention_days" {
  description = "CloudWatch log retention."
  type        = number
  default     = 14
}
