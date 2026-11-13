variable "region" {
  description = "AWS region."
  type        = string
  default     = "eu-central-1"
}

variable "image_tag" {
  description = "Git SHA of the images to deploy. Null creates only the base."
  type        = string
  default     = null
}
