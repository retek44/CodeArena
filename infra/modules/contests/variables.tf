variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-dev."
  type        = string
}

variable "permissions_boundary_arn" {
  description = "Workload permissions boundary (created by infra/bootstrap)."
  type        = string
}

variable "vpc_id" {
  description = "VPC of the cache."
  type        = string
}

variable "subnet_ids" {
  description = "Isolated subnets for the Valkey cache (at least two AZs)."
  type        = list(string)
}

variable "task" {
  description = "Contest lifecycle task of the backend module (output contest)."
  type = object({
    task_definition_arn = string
    task_family         = string
    pass_role_arns      = list(string)
    cluster_arn         = string
    subnet_ids          = list(string)
    security_group_id   = string
  })
}

variable "state_machine_name" {
  description = "Name of the lifecycle state machine (the seed derives its ARN from it)."
  type        = string
}

variable "schedule_group" {
  description = "EventBridge Scheduler group of the contest schedules."
  type        = string
}

variable "scheduler_role_name" {
  description = "Role EventBridge Scheduler assumes to start the state machine."
  type        = string
}

variable "log_retention_days" {
  description = "CloudWatch log retention."
  type        = number
  default     = 14
}
