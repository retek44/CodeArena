output "vpc_id" {
  description = "VPC shared by the preview environments."
  value       = module.network.vpc_id
}
