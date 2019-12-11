locals {
  tags = {
    Name = var.name
  }
}

data "aws_availability_zones" "available" {
  state = "available"
}

resource "aws_vpc" "nodes" {
  cidr_block           = var.cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(local.tags, {
    "kubernetes.io/cluster/${var.name}" : "shared"
  })
}

resource "aws_internet_gateway" "nodes" {
  vpc_id = aws_vpc.nodes.id

  tags = local.tags
}

resource "aws_subnet" "public" {
  count = 3

  availability_zone = data.aws_availability_zones.available.names[count.index]
  cidr_block        = cidrsubnet(var.cidr, 4, count.index)
  vpc_id            = aws_vpc.nodes.id

  tags = merge(local.tags, {
    Name = "${var.name} public ${count.index}"
    "kubernetes.io/cluster/${var.name}" : "shared"
    "kubernetes.io/role/elb" : ""
  })
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.nodes.id

  tags = merge(local.tags, {
    Name = "${var.name} public"
  })
}

resource "aws_route" "public-default" {
  depends_on = [
    aws_internet_gateway.nodes,
    aws_route_table.public,
  ]

  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.nodes.id
  route_table_id         = aws_route_table.public.id
}

resource "aws_route_table_association" "public" {
  count = 3

  route_table_id = aws_route_table.public.id
  subnet_id      = aws_subnet.public[count.index].id
}

resource "aws_subnet" "private" {
  count = 3

  availability_zone = data.aws_availability_zones.available.names[count.index]
  cidr_block        = cidrsubnet(var.cidr, 2, count.index + 1)
  vpc_id            = aws_vpc.nodes.id

  tags = merge(local.tags, {
    Name = "${var.name} private ${count.index}"
    "kubernetes.io/cluster/${var.name}" : "shared"
    "kubernetes.io/role/internal-elb" : ""
  })
}

resource "aws_eip" "nat" {
  count = 3

  vpc = true

  tags = merge(local.tags, {
    Name = "${var.name} nat ${count.index}"
  })
}

resource "aws_nat_gateway" "private" {
  count = 3

  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id

  tags = merge(local.tags, {
    Name = "${var.name} ${count.index}"
  })
}

resource "aws_route_table" "private" {
  count = 3

  vpc_id = aws_vpc.nodes.id

  tags = merge(local.tags, {
    Name = "${var.name} private ${count.index}"
  })
}

resource "aws_route" "private-default" {
  depends_on = [
    aws_internet_gateway.nodes,
    aws_route_table.private,
  ]

  count = 3

  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.private[count.index].id
  route_table_id         = aws_route_table.private[count.index].id
}

resource "aws_route_table_association" "private" {
  count = 3

  route_table_id = aws_route_table.private[count.index].id
  subnet_id      = aws_subnet.private[count.index].id
}
