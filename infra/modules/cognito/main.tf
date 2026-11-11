# User pool for CodeArena users. The React SPA signs in with the authorization code
# flow + PKCE (public client, no secret); the API verifies Cognito access tokens (JWT).
# Essentials tier: the first 10 000 monthly active users are free.

resource "aws_cognito_user_pool" "this" {
  # Threat protection (advanced security) needs the Plus tier, which is not free; not enabled.
  name                = "${var.name_prefix}-users"
  user_pool_tier      = "ESSENTIALS"
  deletion_protection = var.deletion_protection ? "ACTIVE" : "INACTIVE"

  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]

  username_configuration {
    case_sensitive = false
  }

  password_policy {
    minimum_length                   = 12
    require_lowercase                = true
    require_uppercase                = true
    require_numbers                  = true
    require_symbols                  = false
    temporary_password_validity_days = 3
  }

  mfa_configuration = "OPTIONAL"

  software_token_mfa_configuration {
    enabled = true
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  # Cognito's built-in email sender: free, limited to a few emails per day. Enough for a demo.
  email_configuration {
    email_sending_account = "COGNITO_DEFAULT"
  }

  admin_create_user_config {
    allow_admin_create_user_only = var.admin_create_user_only
  }
}

resource "aws_cognito_user_pool_client" "spa" {
  name         = "${var.name_prefix}-spa"
  user_pool_id = aws_cognito_user_pool.this.id

  # Public client: browsers cannot keep a secret; PKCE protects the code exchange.
  generate_secret                      = false
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = var.callback_urls
  logout_urls                          = var.logout_urls

  explicit_auth_flows = ["ALLOW_REFRESH_TOKEN_AUTH", "ALLOW_USER_SRP_AUTH"]

  access_token_validity  = 60
  id_token_validity      = 60
  refresh_token_validity = 30
  token_validity_units {
    access_token  = "minutes"
    id_token      = "minutes"
    refresh_token = "days"
  }

  prevent_user_existence_errors = "ENABLED"
  enable_token_revocation       = true
}

resource "aws_cognito_user_pool_domain" "this" {
  # Prefix domain on amazoncognito.com for the managed login pages.
  domain                = var.domain_prefix
  user_pool_id          = aws_cognito_user_pool.this.id
  managed_login_version = 2
}

resource "aws_cognito_managed_login_branding" "spa" {
  user_pool_id = aws_cognito_user_pool.this.id
  client_id    = aws_cognito_user_pool_client.spa.id

  use_cognito_provided_values = true
}
