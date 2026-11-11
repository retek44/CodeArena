variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-dev."
  type        = string
}

variable "language" {
  description = "Language the runner image supports, for example python."
  type        = string
}

variable "image_uri" {
  description = "Container image with tag, from the shared ECR repository."
  type        = string
}

variable "vpc_id" {
  description = "VPC of the isolated subnets."
  type        = string
}

variable "subnet_ids" {
  description = "Isolated subnets (no route out of the VPC)."
  type        = list(string)
}

variable "permissions_boundary_arn" {
  description = "Workload permissions boundary (created by infra/bootstrap)."
  type        = string
}

variable "memory_mb" {
  description = "Lambda memory. CPU share grows with memory; 1024 MB keeps timings stable."
  type        = number
  default     = 1024
}

variable "timeout_seconds" {
  description = "Lambda timeout for one submission (all test cases)."
  type        = number
  default     = 60
}

variable "reserved_concurrency" {
  description = "Reserved concurrency; -1 means no reservation."
  type        = number
  default     = -1
}

variable "log_retention_days" {
  description = "CloudWatch log retention."
  type        = number
  default     = 14
}
