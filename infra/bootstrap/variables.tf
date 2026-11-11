variable "region" {
  description = "AWS region for all CodeArena resources."
  type        = string
  default     = "eu-central-1"
}

variable "project" {
  description = "Project name. Used as the prefix of every resource name."
  type        = string
  default     = "codearena"
}

variable "github_owner" {
  description = "GitHub user or organization that owns the repository."
  type        = string
}

variable "github_repo" {
  description = "GitHub repository name."
  type        = string
  default     = "CodeArena"
}

variable "environments" {
  description = "Deployment targets. Each one gets a GitHub OIDC deploy role bound to the GitHub Environment of the same name. \"shared\" holds account-wide resources (ECR repositories)."
  type        = list(string)
  default     = ["shared", "preview", "dev", "staging", "prod"]
}

variable "create_github_oidc_provider" {
  description = "Set to false if the account already has the token.actions.githubusercontent.com OIDC provider."
  type        = bool
  default     = true
}

variable "budget_limit_usd" {
  description = "Monthly cost budget in USD."
  type        = number
  default     = 50
}

variable "budget_notification_emails" {
  description = "Email addresses that receive budget alerts."
  type        = list(string)

  validation {
    condition     = length(var.budget_notification_emails) > 0
    error_message = "Give at least one email address for budget alerts."
  }
}
