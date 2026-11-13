# Application Load Balancer. Only CloudFront can reach it:
#   - the security group allows the CloudFront origin-facing prefix list only;
#   - a secret header added by CloudFront must match, otherwise the ALB answers 403.
# There is no custom domain, so CloudFront talks to the ALB over HTTP (ADR-0012).

data "aws_ec2_managed_prefix_list" "cloudfront" {
  name = "com.amazonaws.global.cloudfront.origin-facing"
}

resource "random_password" "origin_verify" {
  length  = 48
  special = false
}

resource "aws_security_group" "alb" {
  name                   = "${var.name_prefix}-alb"
  description            = "ALB: HTTP from CloudFront only"
  vpc_id                 = var.vpc_id
  revoke_rules_on_delete = true
}

resource "aws_vpc_security_group_ingress_rule" "alb_from_cloudfront" {
  #checkov:skip=CKV_AWS_260:The source is the CloudFront origin-facing prefix list, not 0.0.0.0/0.
  security_group_id = aws_security_group.alb.id
  prefix_list_id    = data.aws_ec2_managed_prefix_list.cloudfront.id
  ip_protocol       = "tcp"
  from_port         = 80
  to_port           = 80
  description       = "HTTP from CloudFront edge locations"
}

resource "aws_vpc_security_group_egress_rule" "alb_to_api" {
  security_group_id            = aws_security_group.alb.id
  referenced_security_group_id = aws_security_group.api.id
  ip_protocol                  = "tcp"
  from_port                    = 8080
  to_port                      = 8080
  description                  = "To the API tasks"
}

resource "aws_lb" "this" {
  #checkov:skip=CKV_AWS_91:Access logs need a log bucket; CloudFront and the API log every request already.
  #checkov:skip=CKV_AWS_150:Disposable demo environments must be destroyable without manual steps.
  #checkov:skip=CKV2_AWS_28:WAF costs at least 5 USD per month; the ALB is reachable from CloudFront only.
  #checkov:skip=CKV2_AWS_20:Viewers are redirected to HTTPS by CloudFront; the ALB has no certificate (ADR-0012).
  name                       = "${var.name_prefix}-alb"
  load_balancer_type         = "application"
  internal                   = false
  subnets                    = var.public_subnet_ids
  security_groups            = [aws_security_group.alb.id]
  drop_invalid_header_fields = true
  idle_timeout               = 30
}

resource "aws_lb_target_group" "api" {
  #checkov:skip=CKV_AWS_378:ALB to task traffic stays inside the VPC; TLS terminates at CloudFront (ADR-0012).
  name                 = "${var.name_prefix}-api"
  port                 = 8080
  protocol             = "HTTP"
  target_type          = "ip"
  vpc_id               = var.vpc_id
  deregistration_delay = 15

  health_check {
    path                = "/healthz"
    matcher             = "200"
    interval            = 15
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

resource "aws_lb_listener" "http" {
  #checkov:skip=CKV_AWS_2:No custom domain for an ACM certificate; viewers use HTTPS at CloudFront (ADR-0012).
  #checkov:skip=CKV_AWS_103:TLS terminates at CloudFront (ADR-0012).
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP" # nosemgrep: terraform.aws.security.insecure-load-balancer-tls-version.insecure-load-balancer-tls-version -- TLS ends at CloudFront (ADR-0012)

  default_action {
    type = "fixed-response"

    fixed_response {
      content_type = "text/plain"
      message_body = "Forbidden"
      status_code  = "403"
    }
  }
}

resource "aws_lb_listener_rule" "api" {
  listener_arn = aws_lb_listener.http.arn
  priority     = 10

  condition {
    http_header {
      http_header_name = "X-Origin-Verify"
      values           = [random_password.origin_verify.result]
    }
  }

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }
}
