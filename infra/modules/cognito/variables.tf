variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-dev."
  type        = string
}

variable "domain_prefix" {
  description = "Prefix of the managed login domain (<prefix>.auth.<region>.amazoncognito.com). Must be globally unique."
  type        = string
}

variable "callback_urls" {
  description = "Allowed redirect URLs after sign-in."
  type        = list(string)
}

variable "logout_urls" {
  description = "Allowed redirect URLs after sign-out."
  type        = list(string)
}

variable "admin_create_user_only" {
  description = "Disable self sign-up (only admins create users)."
  type        = bool
  default     = false
}

variable "deletion_protection" {
  description = "Protect the user pool from deletion (enable for prod)."
  type        = bool
  default     = false
}
