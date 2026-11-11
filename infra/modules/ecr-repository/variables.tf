variable "name" {
  description = "Repository name, for example codearena-runner-python."
  type        = string
}

variable "keep_images" {
  description = "Number of images to keep; older ones expire."
  type        = number
  default     = 20
}

variable "force_delete" {
  description = "Allow terraform destroy to delete the repository with images in it."
  type        = bool
  default     = false
}
