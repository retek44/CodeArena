# Monthly cost budget. With the defaults (limit 50 USD) alerts fire at 10, 25 and 50 USD
# actual spend, and when the month's forecast goes over 50 USD.

resource "aws_budgets_budget" "monthly" {
  name         = var.name
  budget_type  = "COST"
  limit_amount = format("%.2f", var.limit_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  dynamic "notification" {
    for_each = toset(var.actual_threshold_percents)

    content {
      comparison_operator        = "GREATER_THAN"
      threshold                  = notification.value
      threshold_type             = "PERCENTAGE"
      notification_type          = "ACTUAL"
      subscriber_email_addresses = var.notification_emails
    }
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = var.forecast_threshold_percent
    threshold_type             = "PERCENTAGE"
    notification_type          = "FORECASTED"
    subscriber_email_addresses = var.notification_emails
  }
}
