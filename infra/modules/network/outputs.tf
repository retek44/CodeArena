output "vpc_id" {
  description = "VPC ID."
  value       = aws_vpc.this.id
}

output "public_subnet_ids" {
  description = "Subnets with a route to the internet gateway."
  value       = aws_subnet.public[*].id
}

output "isolated_subnet_ids" {
  description = "Subnets without any route out of the VPC."
  value       = aws_subnet.isolated[*].id
}

output "isolated_route_table_id" {
  description = "Route table of the isolated subnets (for gateway endpoints later)."
  value       = aws_route_table.isolated.id
}
