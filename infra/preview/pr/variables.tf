variable "region" {
  description = "AWS region."
  type        = string
  default     = "eu-central-1"
}

variable "pr_number" {
  description = "Pull request number; resources are named codearena-pr-<n>-*."
  type        = number

  validation {
    condition     = var.pr_number >= 1 && var.pr_number <= 99999 && floor(var.pr_number) == var.pr_number
    error_message = "pr_number must be a pull request number (1..99999)."
  }
}

variable "image_tag" {
  description = "Git SHA of the images to deploy. Null creates only the base (queue, Cognito)."
  type        = string
  default     = null
}
