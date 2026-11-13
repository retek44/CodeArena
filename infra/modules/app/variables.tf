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
  description = "VPC CIDR; distinct per environment."
  type        = string
}

variable "db_instance_class" {
  description = "RDS instance class."
  type        = string
  default     = "db.t4g.micro"
}

variable "api_desired_count" {
  description = "API tasks."
  type        = number
  default     = 1
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

variable "hints_enabled" {
  description = "Deploy the AI hint (Bedrock). Needs model access in the account (docs/SETUP.md); without it the API answers 503 for hints."
  type        = bool
  default     = true
}

variable "hint_model_id" {
  description = "Bedrock model or cross-region inference profile for hints."
  type        = string
  default     = "eu.anthropic.claude-haiku-5-5"
}
