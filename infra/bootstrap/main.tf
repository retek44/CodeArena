# One-time, account-level foundation:
#   - S3 bucket for Terraform state (native S3 locking, no DynamoDB table)
#   - GitHub Actions OIDC provider and CI roles
#   - Monthly cost budget with email alerts
# Apply it manually with admin credentials. CI never touches this stack.

provider "aws" {
  region = var.region

  default_tags {
    tags = local.tags
  }
}

data "aws_caller_identity" "current" {}

locals {
  account_id        = data.aws_caller_identity.current.account_id
  state_bucket_name = "${var.project}-tfstate-${local.account_id}-${var.region}"

  tags = {
    Project    = var.project
    Stack      = "bootstrap"
    ManagedBy  = "terraform"
    Repository = "${var.github_owner}/${var.github_repo}"
  }
}

# --- Terraform state bucket ---------------------------------------------------

resource "aws_s3_bucket" "tfstate" {
  #checkov:skip=CKV_AWS_18:Access logs need a second bucket; for a single-user project, versioning already keeps every state change.
  #checkov:skip=CKV_AWS_144:Cross-region replication doubles cost; versioning is enough to recover state.
  #checkov:skip=CKV_AWS_145:SSE-S3 is used on purpose; a customer managed KMS key costs 1 USD per month.
  #checkov:skip=CKV2_AWS_62:No consumer for event notifications on the state bucket.
  bucket = local.state_bucket_name

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_s3_bucket_ownership_controls" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_public_access_block" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id

  versioning_configuration {
    status = "Enabled"
  }
}

# SSE-S3 instead of a customer managed KMS key: free, and enough for a single-account project.
resource "aws_s3_bucket_server_side_encryption_configuration" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id

  rule {
    id     = "expire-old-state-versions"
    status = "Enabled"

    filter {}

    noncurrent_version_expiration {
      noncurrent_days = 90
    }

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }

  depends_on = [aws_s3_bucket_versioning.tfstate]
}

data "aws_iam_policy_document" "tfstate_bucket" {
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    resources = [
      aws_s3_bucket.tfstate.arn,
      "${aws_s3_bucket.tfstate.arn}/*",
    ]

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  policy = data.aws_iam_policy_document.tfstate_bucket.json

  depends_on = [aws_s3_bucket_public_access_block.tfstate]
}

# --- CI access and cost guardrails --------------------------------------------

module "github_oidc" {
  source = "../modules/github-oidc"

  project          = var.project
  github_owner     = var.github_owner
  github_repo      = var.github_repo
  environments     = var.environments
  create_provider  = var.create_github_oidc_provider
  state_bucket_arn = aws_s3_bucket.tfstate.arn
}

module "budget" {
  source = "../modules/budget"

  name                = "${var.project}-monthly"
  limit_usd           = var.budget_limit_usd
  notification_emails = var.budget_notification_emails
}
