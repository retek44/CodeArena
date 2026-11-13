variable "name" {
  description = "Queue name, for example codearena-dev-submissions. The DLQ gets a -dlq suffix."
  type        = string
}

variable "visibility_timeout_seconds" {
  description = "How long a received message stays hidden. Must match the worker lease (WORKER_LEASE) and exceed the runner Lambda timeout."
  type        = number
  default     = 120
}

variable "max_receive_count" {
  description = "Deliveries before a message moves to the dead-letter queue."
  type        = number
  default     = 5
}

variable "dlq_retention_seconds" {
  description = "How long dead letters are kept for investigation."
  type        = number
  default     = 1209600 # 14 days
}
