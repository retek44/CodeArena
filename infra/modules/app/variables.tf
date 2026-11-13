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
