data "aws_route_table" "public" {
  count     = length(var.public_subnets_ids)
  subnet_id = var.public_subnets_ids[count.index]
}

data "aws_route_table" "private" {
  count     = length(var.private_subnets_ids)
  subnet_id = var.private_subnets_ids[count.index]
}

data "aws_nat_gateway" "private" {
  count = var.high_availability ? length(var.private_subnet_ids) : 1
  subnet_id = var.private_subnets_ids[count.index]
}