variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-dev."
  type        = string
}

variable "api_origin_domain" {
  description = "ALB host name for /api/*."
  type        = string
}

variable "api_origin_verify_header" {
  description = "Secret X-Origin-Verify value the ALB requires."
  type        = string
  sensitive   = true
}

variable "content_security_policy" {
  description = "Content-Security-Policy for the SPA. Monaco needs inline styles and same-origin workers."
  type        = string
}
