variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-dev."
  type        = string
}

variable "cidr_block" {
  description = "VPC CIDR. Subnets are /24: public from .0, isolated from .100."
  type        = string
  default     = "10.20.0.0/16"
}

variable "availability_zones" {
  description = "Availability zones, for example [\"eu-central-1a\", \"eu-central-1b\"]. RDS and ALB need at least 2."
  type        = list(string)

  validation {
    condition     = length(var.availability_zones) >= 2 && length(var.availability_zones) <= 3
    error_message = "Give 2 or 3 availability zones."
  }
}
