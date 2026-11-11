# VPC without NAT Gateway (ADR-0005):
#   public subnets   -> route to the internet gateway (ALB, Fargate tasks with public IP)
#   isolated subnets -> no route out of the VPC (RDS, ElastiCache, runner Lambdas)

locals {
  # Zones are pinned explicitly, so the network never changes when AWS adds a zone.
  azs      = var.availability_zones
  az_count = length(var.availability_zones)
}

resource "aws_vpc" "this" {
  #checkov:skip=CKV2_AWS_11:Flow logs cost money per GB; enable for production, not for a demo environment.
  cidr_block           = var.cidr_block
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${var.name_prefix}-vpc"
  }
}

# Take over the default security group and leave it without rules.
resource "aws_default_security_group" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.name_prefix}-default-deny"
  }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.name_prefix}-igw"
  }
}

resource "aws_subnet" "public" {
  count = local.az_count

  vpc_id            = aws_vpc.this.id
  availability_zone = local.azs[count.index]
  cidr_block        = cidrsubnet(var.cidr_block, 8, count.index)
  # Tasks request a public IP explicitly; nothing gets one by default.
  map_public_ip_on_launch = false

  tags = {
    Name = "${var.name_prefix}-public-${local.azs[count.index]}"
    Tier = "public"
  }
}

resource "aws_subnet" "isolated" {
  count = local.az_count

  vpc_id            = aws_vpc.this.id
  availability_zone = local.azs[count.index]
  cidr_block        = cidrsubnet(var.cidr_block, 8, 100 + count.index)

  tags = {
    Name = "${var.name_prefix}-isolated-${local.azs[count.index]}"
    Tier = "isolated"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.name_prefix}-public"
  }
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.this.id
}

resource "aws_route_table_association" "public" {
  count = local.az_count

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Only the implicit local route: traffic cannot leave the VPC.
resource "aws_route_table" "isolated" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.name_prefix}-isolated"
  }
}

resource "aws_route_table_association" "isolated" {
  count = local.az_count

  subnet_id      = aws_subnet.isolated[count.index].id
  route_table_id = aws_route_table.isolated.id
}
