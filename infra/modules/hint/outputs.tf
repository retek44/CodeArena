output "function_arn" {
  description = "Hint Lambda the API invokes (HINT_FUNCTION)."
  value       = aws_lambda_function.hint.arn
}

output "function_name" {
  description = "Hint Lambda name (the LLM evaluation invokes it)."
  value       = aws_lambda_function.hint.function_name
}

output "guardrail_id" {
  description = "Bedrock guardrail ID."
  value       = aws_bedrock_guardrail.hint.guardrail_id
}

output "guardrail_version" {
  description = "Guardrail version the function uses."
  value       = aws_bedrock_guardrail_version.hint.version
}

output "model_id" {
  description = "Bedrock model or inference profile ID."
  value       = var.model_id
}
