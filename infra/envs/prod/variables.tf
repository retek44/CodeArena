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

variable "alert_email" {
  description = "E-mail for alarm notifications (GitHub variable ALERT_EMAIL). Empty: none."
  type        = string
  default     = ""
  nullable    = false
}

variable "contest_email_from" {
  description = "Verified SES sender for contest result e-mails (repository variable CONTEST_EMAIL_FROM); empty sends none."
  type        = string
  default     = ""
  nullable    = false
}
