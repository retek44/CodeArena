variable "name_prefix" {
  description = "Prefix for resource names, for example codearena-dev."
  type        = string
}

variable "vpc_id" {
  description = "VPC of the database."
  type        = string
}

variable "subnet_ids" {
  description = "Isolated subnets (no route out of the VPC), at least two availability zones."
  type        = list(string)
}

variable "client_security_group_ids" {
  description = "Security groups allowed to connect on 5432 (API, worker, migration task)."
  type        = list(string)
}

variable "instance_class" {
  description = "Instance class. db.t4g.micro is the cheapest Graviton class."
  type        = string
  default     = "db.t4g.micro"
}

variable "allocated_storage_gb" {
  description = "Storage in GB (gp3)."
  type        = number
  default     = 20
}

variable "backup_retention_days" {
  description = "Automated backup retention. Dev and preview use 1; prod 7 or more."
  type        = number
  default     = 1
}

variable "deletion_protection" {
  description = "Block deletion (enable for prod)."
  type        = bool
  default     = false
}

variable "skip_final_snapshot" {
  description = "Destroy without a final snapshot (dev and preview environments are disposable)."
  type        = bool
  default     = true
}
