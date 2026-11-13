variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-dev."
  type        = string
}

variable "permissions_boundary_arn" {
  description = "Workload permissions boundary (created by infra/bootstrap)."
  type        = string
}

variable "model_id" {
  description = "Bedrock model or cross-region inference profile ID. Model access must be enabled once per account (docs/SETUP.md)."
  type        = string
  default     = "eu.anthropic.claude-haiku-5-5"
}

variable "log_retention_days" {
  description = "CloudWatch log retention."
  type        = number
  default     = 14
}
