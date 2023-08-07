data "aws_route_table" "public" {
  count     = length(var.public_subnets_ids)
  subnet_id = var.public_subnets_ids[count.index]
}

data "aws_route_table" "private" {
  count     = length(var.private_subnets_ids)
  subnet_id = var.private_subnets_ids[count.index]
}

data "aws_subnet" "private_subnet_details" {
  count = length(local.private_subnets_ids)
  id    = local.private_subnets_ids[count.index]
}

data "aws_subnet" "public_subnet_details" {
  count = length(local.public_subnets_ids)
  id    = local.public_subnets_ids[count.index]
}
