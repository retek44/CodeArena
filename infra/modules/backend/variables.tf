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

variable "runner_function_arn" {
  description = "Python runner Lambda the worker invokes."
  type        = string
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
