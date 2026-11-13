output "domain_name" {
  description = "Public host name of the site (https://<domain_name>)."
  value       = aws_cloudfront_distribution.this.domain_name
}

output "distribution_id" {
  description = "Distribution ID (cache invalidation after a deploy)."
  value       = aws_cloudfront_distribution.this.id
}

output "bucket" {
  description = "Bucket for the SPA build."
  value       = aws_s3_bucket.site.bucket
}
