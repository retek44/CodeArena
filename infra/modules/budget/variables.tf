variable "name" {
  description = "Budget name."
  type        = string
}

variable "limit_usd" {
  description = "Monthly limit in USD."
  type        = number
}

variable "notification_emails" {
  description = "Email addresses that receive the alerts."
  type        = list(string)
}

variable "actual_threshold_percents" {
  description = "Alert when actual spend reaches these percentages of the limit."
  type        = list(number)
  default     = [20, 50, 100]
}

variable "forecast_threshold_percent" {
  description = "Alert when forecasted spend for the month reaches this percentage of the limit."
  type        = number
  default     = 100
}
