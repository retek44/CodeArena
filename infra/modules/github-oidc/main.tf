# GitHub Actions authenticates to AWS with short-lived OIDC tokens. No access keys exist.
#
# Roles:
#   <project>-ci-plan          read-only, for terraform plan on pull requests and main
#   <project>-ci-deploy-<env>  one per GitHub Environment, can only be assumed by a job
#                              that runs in that environment (prod needs a reviewer)
#
# Deploy roles may create IAM roles only with the <project>- prefix and only with the
# workload permissions boundary attached. This blocks privilege escalation through a
# new role with AdministratorAccess.

data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

locals {
  account_id   = data.aws_caller_identity.current.account_id
  partition    = data.aws_partition.current.partition
  repo         = "${var.github_owner}/${var.github_repo}"
  oidc_host    = "token.actions.githubusercontent.com"
  provider_arn = var.create_provider ? aws_iam_openid_connect_provider.github[0].arn : data.aws_iam_openid_connect_provider.github[0].arn

  iam_arn_prefix = "arn:${local.partition}:iam::${local.account_id}"
  boundary_name  = "${var.project}-workload-boundary"
  boundary_arn   = "${local.iam_arn_prefix}:policy/${local.boundary_name}"

  # Each environment writes only its own state keys.
  # "preview" and "shared" (account-wide resources such as ECR repositories) have top-level prefixes.
  state_prefixes = {
    for env in var.environments : env => contains(["preview", "shared"], env) ? "${env}/" : "envs/${env}/"
  }
}

resource "aws_iam_openid_connect_provider" "github" {
  count = var.create_provider ? 1 : 0

  url            = "https://${local.oidc_host}"
  client_id_list = ["sts.amazonaws.com"]
}

data "aws_iam_openid_connect_provider" "github" {
  count = var.create_provider ? 0 : 1

  url = "https://${local.oidc_host}"
}

# --- Plan role ----------------------------------------------------------------

data "aws_iam_policy_document" "plan_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:sub"
      values = [
        "repo:${local.repo}:pull_request",
        "repo:${local.repo}:ref:refs/heads/main",
      ]
    }
  }
}

resource "aws_iam_role" "plan" {
  name                 = "${var.project}-ci-plan"
  description          = "GitHub Actions: read-only terraform plan"
  assume_role_policy   = data.aws_iam_policy_document.plan_trust.json
  max_session_duration = 3600
}

resource "aws_iam_role_policy_attachment" "plan_readonly" {
  role       = aws_iam_role.plan.name
  policy_arn = "arn:${local.partition}:iam::aws:policy/ReadOnlyAccess"
}

data "aws_iam_policy_document" "plan_state" {
  statement {
    sid       = "ListStateBucket"
    actions   = ["s3:ListBucket"]
    resources = [var.state_bucket_arn]
  }

  statement {
    sid       = "ReadState"
    actions   = ["s3:GetObject"]
    resources = ["${var.state_bucket_arn}/*"]
  }

  # terraform plan takes the S3 native lock, so it must write and delete .tflock files.
  statement {
    sid       = "StateLock"
    actions   = ["s3:PutObject", "s3:DeleteObject"]
    resources = ["${var.state_bucket_arn}/*.tflock"]
  }
}

resource "aws_iam_role_policy" "plan_state" {
  name   = "terraform-state"
  role   = aws_iam_role.plan.id
  policy = data.aws_iam_policy_document.plan_state.json
}

# --- Workload permissions boundary --------------------------------------------

data "aws_iam_policy_document" "boundary" {
  statement {
    sid       = "AllowProjectServices"
    actions   = [for s in var.workload_services : "${s}:*"]
    resources = ["*"]
  }

  statement {
    sid       = "AllowPassProjectRoles"
    actions   = ["iam:PassRole", "iam:GetRole"]
    resources = ["${local.iam_arn_prefix}:role/${var.project}-*"]
  }
}

resource "aws_iam_policy" "boundary" {
  name        = local.boundary_name
  description = "Permissions boundary for every IAM role that CI creates."
  policy      = data.aws_iam_policy_document.boundary.json
}

# --- Deploy roles -------------------------------------------------------------

data "aws_iam_policy_document" "deploy_trust" {
  for_each = toset(var.environments)

  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_host}:sub"
      values   = ["repo:${local.repo}:environment:${each.key}"]
    }
  }
}

resource "aws_iam_role" "deploy" {
  for_each = toset(var.environments)

  name                 = "${var.project}-ci-deploy-${each.key}"
  description          = "GitHub Actions: deploy to ${each.key}"
  assume_role_policy   = data.aws_iam_policy_document.deploy_trust[each.key].json
  max_session_duration = 3600
}

resource "aws_iam_role_policy_attachment" "deploy_managed" {
  for_each = {
    for pair in setproduct(var.environments, var.deploy_managed_policy_arns) :
    "${pair[0]}|${pair[1]}" => { env = pair[0], arn = pair[1] }
  }

  role       = aws_iam_role.deploy[each.value.env].name
  policy_arn = each.value.arn
}

data "aws_iam_policy_document" "deploy" {
  #checkov:skip=CKV_AWS_356:Only read-only iam:Get*/List* use "*"; several IAM List APIs support no resource scoping.
  for_each = toset(var.environments)

  statement {
    sid       = "ListStateBucket"
    actions   = ["s3:ListBucket"]
    resources = [var.state_bucket_arn]
  }

  statement {
    sid       = "ReadWriteOwnState"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["${var.state_bucket_arn}/${local.state_prefixes[each.key]}*"]
  }

  statement {
    sid       = "IamRead"
    actions   = ["iam:Get*", "iam:List*"]
    resources = ["*"]
  }

  # Roles can be created or changed only if they carry the workload boundary.
  statement {
    sid = "ManageProjectRolesWithBoundary"
    actions = [
      "iam:CreateRole",
      "iam:PutRolePermissionsBoundary",
      "iam:AttachRolePolicy",
      "iam:DetachRolePolicy",
      "iam:PutRolePolicy",
      "iam:DeleteRolePolicy",
    ]
    resources = ["${local.iam_arn_prefix}:role/${var.project}-*"]

    condition {
      test     = "StringEquals"
      variable = "iam:PermissionsBoundary"
      values   = [local.boundary_arn]
    }
  }

  statement {
    sid = "ManageProjectRoles"
    actions = [
      "iam:DeleteRole",
      "iam:UpdateRole",
      "iam:UpdateRoleDescription",
      "iam:UpdateAssumeRolePolicy",
      "iam:TagRole",
      "iam:UntagRole",
      "iam:PassRole",
    ]
    resources = ["${local.iam_arn_prefix}:role/${var.project}-*"]
  }

  statement {
    sid = "ManageProjectPolicies"
    actions = [
      "iam:CreatePolicy",
      "iam:DeletePolicy",
      "iam:CreatePolicyVersion",
      "iam:DeletePolicyVersion",
      "iam:SetDefaultPolicyVersion",
      "iam:TagPolicy",
      "iam:UntagPolicy",
    ]
    resources = ["${local.iam_arn_prefix}:policy/${var.project}-*"]
  }

  statement {
    sid       = "ServiceLinkedRoles"
    actions   = ["iam:CreateServiceLinkedRole"]
    resources = ["${local.iam_arn_prefix}:role/aws-service-role/*"]
  }

  # The boundary matches the project prefix, so protect it explicitly.
  statement {
    sid    = "ProtectBoundary"
    effect = "Deny"
    actions = [
      "iam:CreatePolicyVersion",
      "iam:DeletePolicy",
      "iam:DeletePolicyVersion",
      "iam:SetDefaultPolicyVersion",
    ]
    resources = [local.boundary_arn]
  }

  statement {
    sid       = "NoBoundaryRemoval"
    effect    = "Deny"
    actions   = ["iam:DeleteRolePermissionsBoundary"]
    resources = ["*"]
  }

  # CI roles are managed by the bootstrap stack only. Reads stay allowed.
  statement {
    sid         = "ProtectCiRoles"
    effect      = "Deny"
    not_actions = ["iam:Get*", "iam:List*"]
    resources   = ["${local.iam_arn_prefix}:role/${var.project}-ci-*"]
  }
}

resource "aws_iam_role_policy" "deploy" {
  for_each = toset(var.environments)

  name   = "deploy"
  role   = aws_iam_role.deploy[each.key].id
  policy = data.aws_iam_policy_document.deploy[each.key].json
}
