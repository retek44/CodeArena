output "name_prefix" {
  description = "Prefix for every resource name in this environment."
  value       = local.name_prefix
}

output "vpc_id" {
  description = "VPC ID."
  value       = module.network.vpc_id
}

output "runner_python_function_name" {
  description = "Python runner function, if deployed."
  value       = one(module.runner_python[*].function_name)
}
