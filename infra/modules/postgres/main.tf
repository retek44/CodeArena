# PostgreSQL on RDS in the isolated subnets. RDS generates and rotates the master
# password in Secrets Manager (manage_master_user_password), so no password exists
# in Terraform code, variables or state.

resource "aws_db_subnet_group" "this" {
  name       = "${var.name_prefix}-db"
  subnet_ids = var.subnet_ids
}

resource "aws_security_group" "db" {
  name                   = "${var.name_prefix}-db"
  description            = "PostgreSQL: only from the application security groups"
  vpc_id                 = var.vpc_id
  revoke_rules_on_delete = true

  tags = {
    Name = "${var.name_prefix}-db"
  }
}

resource "aws_vpc_security_group_ingress_rule" "clients" {
  for_each = var.client_security_group_ids

  security_group_id            = aws_security_group.db.id
  referenced_security_group_id = each.value
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
  description                  = "PostgreSQL from application tasks"
}

resource "aws_db_parameter_group" "this" {
  name   = "${var.name_prefix}-postgres18"
  family = "postgres18"

  # Reject unencrypted connections (the default for 15+, stated explicitly).
  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }

  parameter {
    name  = "log_min_duration_statement"
    value = "500"
  }
}

resource "aws_db_instance" "this" {
  #checkov:skip=CKV_AWS_157:Single-AZ on purpose for cost; prod would use Multi-AZ.
  #checkov:skip=CKV_AWS_118:Enhanced monitoring adds cost; CloudWatch metrics and Performance Insights cover a demo.
  #checkov:skip=CKV_AWS_353:Performance Insights is not available for db.t4g.micro.
  #checkov:skip=CKV2_AWS_30:Query logging is enabled for slow statements (log_min_duration_statement).
  identifier     = "${var.name_prefix}-db"
  engine         = "postgres"
  engine_version = "18"
  instance_class = var.instance_class

  db_name                     = "codearena"
  username                    = "codearena"
  manage_master_user_password = true

  allocated_storage = var.allocated_storage_gb
  storage_type      = "gp3"
  storage_encrypted = true

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.db.id]
  parameter_group_name   = aws_db_parameter_group.this.name
  publicly_accessible    = false
  multi_az               = false

  iam_database_authentication_enabled = true
  enabled_cloudwatch_logs_exports     = ["postgresql"]
  auto_minor_version_upgrade          = true
  copy_tags_to_snapshot               = true
  backup_retention_period             = var.backup_retention_days
  deletion_protection                 = var.deletion_protection
  skip_final_snapshot                 = var.skip_final_snapshot
  final_snapshot_identifier           = var.skip_final_snapshot ? null : "${var.name_prefix}-db-final"
  apply_immediately                   = true
}
