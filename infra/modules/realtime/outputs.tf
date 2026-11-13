output "ws_url" {
  description = "wss:// URL the SPA connects to (returned with every ticket)."
  value       = "${aws_apigatewayv2_api.ws.api_endpoint}/${local.stage}"
}

output "ws_origin" {
  description = "Origin of the WebSocket API for the CSP connect-src directive."
  value       = aws_apigatewayv2_api.ws.api_endpoint
}

output "table" {
  description = "DynamoDB table of tickets, connections and subscriptions."
  value = {
    name = aws_dynamodb_table.ws.name
    arn  = aws_dynamodb_table.ws.arn
  }
}

output "event_bus" {
  description = "EventBridge bus of the judge events."
  value = {
    name = aws_cloudwatch_event_bus.this.name
    arn  = aws_cloudwatch_event_bus.this.arn
  }
}

output "function_names" {
  description = "WebSocket (ws) and push Lambda names."
  value       = { for key, fn in aws_lambda_function.this : key => fn.function_name }
}
