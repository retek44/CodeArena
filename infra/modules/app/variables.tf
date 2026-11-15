variable "env" {
  description = "Environment name: dev, staging, prod or pr-<number>."
  type        = string

  validation {
    condition     = can(regex("^(dev|staging|prod|pr-[0-9]+)$", var.env))
    error_message = "env must be dev, staging, prod or pr-<number>."
  }
}

variable "image_tag" {
  description = "Git SHA of the images in ECR. Null creates only the base (network, queue, Cognito)."
  type        = string
  default     = null
}

variable "cidr_block" {
  description = "VPC CIDR; distinct per environment. Unused with network."
  type        = string
  default     = null
}

variable "network" {
  description = "Existing VPC to deploy into instead of creating one (preview environments, ADR-0019)."
  type = object({
    vpc_id              = string
    public_subnet_ids   = list(string)
    isolated_subnet_ids = list(string)
  })
  default = null

  validation {
    condition     = var.network != null || var.cidr_block != null
    error_message = "Set cidr_block for an own VPC, or network to use an existing one."
  }
}

variable "db_instance_class" {
  description = "RDS instance class."
  type        = string
  default     = "db.t4g.micro"
}

variable "tracing" {
  description = "OpenTelemetry tracing of the API and worker to X-Ray (ADR-0018); sample_ratio is the share of new traces kept."
  type = object({
    enabled      = bool
    sample_ratio = number
  })
  default = { enabled = true, sample_ratio = 1 }
}

variable "api_desired_count" {
  description = "API tasks."
  type        = number
  default     = 1
}

variable "api_bake_time_minutes" {
  description = "Blue/green bake time of the API: minutes the old tasks stay after the switch, ready for an instant rollback."
  type        = number
  default     = 0

  validation {
    condition     = var.api_bake_time_minutes >= 0 && var.api_bake_time_minutes <= 1440
    error_message = "ECS accepts 0 to 1440 minutes."
  }
}

variable "runner_canary" {
  description = "Shift runner Lambda versions with a CodeDeploy canary (10 % for 5 minutes, smoke test first) instead of at once."
  type        = bool
  default     = false
}

variable "worker_max_count" {
  description = "Maximum worker tasks (scales to zero when idle)."
  type        = number
  default     = 2
}

variable "protect_data" {
  description = "Deletion protection, backups and final snapshot (prod)."
  type        = bool
  default     = false
}

variable "allow_localhost_sign_in" {
  description = "Allow Cognito redirects to http://localhost:5173 (dev only)."
  type        = bool
  default     = false
}

variable "alert_email" {
  description = "E-mail address for the alerts topic. Empty: no subscription (alarms still show in CloudWatch). AWS sends a confirmation mail after each create."
  type        = string
  default     = ""
  nullable    = false
}

variable "canary_enabled" {
  description = "Run the Synthetics canary (sign in -> submit -> AC) every 30 minutes; about 2 USD a month while the environment exists."
  type        = bool
  default     = false
}

variable "contest_email_from" {
  description = "Verified SES sender for contest result e-mails; empty sends none (docs/SETUP.md)."
  type        = string
  default     = ""
  nullable    = false
}
