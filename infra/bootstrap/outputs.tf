output "account_id" {
  description = "AWS account ID. Store it as the AWS_ACCOUNT_ID GitHub repository variable."
  value       = local.account_id
}

output "region" {
  description = "AWS region. Store it as the AWS_REGION GitHub repository variable."
  value       = var.region
}

output "state_bucket" {
  description = "Terraform state bucket. Store it as the TF_STATE_BUCKET GitHub repository variable."
  value       = aws_s3_bucket.tfstate.bucket
}

output "plan_role_arn" {
  description = "Read-only role that CI assumes for terraform plan."
  value       = module.github_oidc.plan_role_arn
}

output "deploy_role_arns" {
  description = "Deploy role per environment."
  value       = module.github_oidc.deploy_role_arns
}

output "backend_hcl" {
  description = "Content for infra/backend.hcl (local use, gitignored)."
  value       = <<-EOT
    bucket = "${aws_s3_bucket.tfstate.bucket}"
    region = "${var.region}"
  EOT
}
