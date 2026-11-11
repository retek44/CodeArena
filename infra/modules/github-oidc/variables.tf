variable "project" {
  description = "Project name. Prefix of the CI role names and of every IAM role that the deploy roles may manage."
  type        = string
}

variable "github_owner" {
  description = "GitHub user or organization (case-sensitive, as GitHub puts it in the OIDC sub claim)."
  type        = string
}

variable "github_repo" {
  description = "GitHub repository name (case-sensitive)."
  type        = string
}

variable "environments" {
  description = "GitHub Environments that get a deploy role."
  type        = list(string)
}

variable "create_provider" {
  description = "Create the GitHub OIDC provider. Set to false if the account already has one."
  type        = bool
  default     = true
}

variable "state_bucket_arn" {
  description = "ARN of the Terraform state bucket."
  type        = string
}

variable "deploy_managed_policy_arns" {
  description = "AWS managed policies attached to every deploy role. IAM access is granted separately and limited by a permissions boundary."
  type        = list(string)
  default     = ["arn:aws:iam::aws:policy/PowerUserAccess"]
}

variable "workload_services" {
  description = "Service prefixes that roles created by CI (Lambda, ECS task, Step Functions roles) may use. Enforced by the permissions boundary."
  type        = list(string)
  default = [
    "apigateway", "application-autoscaling", "bedrock", "cloudwatch", "codedeploy", "cognito-idp",
    "dynamodb", "ec2", "ecr", "ecs", "elasticache", "elasticloadbalancing", "events", "execute-api",
    "fis", "kms", "lambda", "logs", "rds", "s3", "scheduler", "secretsmanager", "ses", "sns", "sqs",
    "ssm", "states", "sts", "synthetics", "xray",
  ]
}
