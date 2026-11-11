variable "region" {
  description = "AWS region."
  type        = string
  default     = "eu-central-1"
}

variable "runner_python_image_tag" {
  description = "Tag of the Python runner image in ECR. Null skips the runner function."
  type        = string
  default     = null
}
