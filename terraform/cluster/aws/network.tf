locals {
  tags = merge(var.tags, {
    Name = var.name
    Rack = var.name
  })
  vpc_id = var.vpc_id == "" ? aws_vpc.nodes[0].id : var.vpc_id

  private_subnets_ids  = length(var.private_subnets_ids) == 0 ? aws_subnet.private[*].id : var.private_subnets_ids
  private_route_tables = length(var.private_subnets_ids) == 0 ? aws_route_table.private[*].id : data.aws_route_table.private[*].id
  nat_gateway_ids       = length(var.private_subnets_ids) == 0 ? aws_nat_gateway.private[*].id : data.aws_nat_gateway.private[*].id

  public_subnets_ids  = length(var.public_subnets_ids) == 0 ? aws_subnet.public[*].id : var.public_subnets_ids
  public_route_table  = length(var.public_subnets_ids) == 0 ? aws_route_table.public[0].id : data.aws_route_table.public.id
  internet_gateway_id = var.internet_gateway_id == "" ? aws_internet_gateway.nodes[0].id : var.internet_gateway_id
}

resource "aws_vpc" "nodes" {
  count = var.vpc_id == "" ? 1 : 0

  depends_on = [
    aws_iam_role_policy_attachment.cluster_eks_cluster,
    aws_iam_role_policy_attachment.cluster_eks_service,
    aws_iam_role_policy_attachment.nodes_ecr,
    aws_iam_role_policy_attachment.nodes_eks_cni,
    aws_iam_role_policy_attachment.nodes_eks_worker,
  ]

  cidr_block           = var.cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(local.tags, {
    "kubernetes.io/cluster/${var.name}" : "shared"
  })
}

resource "aws_internet_gateway" "nodes" {
  count  = var.internet_gateway_id == "" ? 1 : 0
  vpc_id = local.vpc_id

  tags = local.tags
}

// workaround for aws eventual consistency API problem
// https://github.com/hashicorp/terraform-provider-aws/issues/13138
resource "null_resource" "wait_vpc_nodes" {
  provisioner "local-exec" {
    command = "sleep 30"
  }

  depends_on = [
    aws_vpc.nodes
  ]

}

resource "aws_subnet" "public" {
  depends_on = [
    null_resource.wait_vpc_nodes
  ]

  count = length(var.public_subnets_ids) == 0 ? local.network_resource_count : 0

  availability_zone       = local.availability_zones[count.index]
  cidr_block              = cidrsubnet(var.cidr, 4, count.index)
  map_public_ip_on_launch = !var.private
  vpc_id                  = local.vpc_id

  tags = merge(local.tags, {
    Name = "${var.name} public ${count.index}"
    "kubernetes.io/cluster/${var.name}" : "shared"
    "kubernetes.io/role/elb" : ""
  })

  timeouts {
    delete = "6h"
  }
}

data "aws_route_table" "public" {
  subnet_id = var.public_subnets_ids[0]
}

resource "aws_route_table" "public" {
  count  = length(var.public_subnets_ids) == 0 ? 1 : 0
  vpc_id = local.vpc_id

  tags = merge(local.tags, {
    Name = "${var.name} public"
  })
}

resource "null_resource" "wait_routes_public" {
  provisioner "local-exec" {
    command = "sleep 30"
  }

  depends_on = [
    local.public_route_table,
    aws_internet_gateway.nodes
  ]

}

resource "aws_route" "public-default" {
  depends_on = [
    null_resource.wait_routes_public
  ]

  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = local.internet_gateway_id
  route_table_id         = local.public_route_table

  timeouts {
    create = "10m"
  }
}

resource "aws_route_table_association" "public" {
  count = local.network_resource_count

  route_table_id = local.public_route_table
  subnet_id      = local.public_subnets_ids[count.index]
}

resource "aws_subnet" "private" {
  depends_on = [
    null_resource.wait_vpc_nodes
  ]

  // If the variable `private_subnets_ids` is empty
  // | if yes, check if `private` is true
  // | | if yes, create private subnets according to `local.network_resource_count`
  // | | else, don't create private subnets
  // else, don't create private subnets
  count = length(var.private_subnets_ids) == 0 ? var.private ? local.network_resource_count : 0 : 0

  availability_zone = local.availability_zones[count.index]
  cidr_block        = cidrsubnet(var.cidr, 2, count.index + 1)
  vpc_id            = local.vpc_id

  tags = merge(local.tags, {
    Name = "${var.name} private ${count.index}"
    "kubernetes.io/cluster/${var.name}" : "shared"
    "kubernetes.io/role/internal-elb" : ""
  })

  timeouts {
    delete = "6h"
  }
}

resource "aws_eip" "nat" {
  count = var.private ? local.network_resource_count : 0

  vpc = true

  tags = merge(local.tags, {
    Name = "${var.name} nat ${count.index}"
  })
}

data "aws_nat_gateway" "private" {
  count  = length(var.private_subnets_ids) == 0 ? 0 : 1
  vpc_id = local.vpc_id
  #subnet_id = var.public_subnets_ids[0]
}

resource "aws_nat_gateway" "private" {
  count = length(var.private_subnets_ids) == 0 ? var.private ? local.network_resource_count : 0 : 0

  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = local.public_subnets_ids[count.index]

  tags = merge(local.tags, {
    Name = "${var.name} ${count.index}"
  })
}

data "aws_route_table" "private" {
  count     = length(var.private_subnets_ids)
  subnet_id = var.private_subnets_ids[count.index]
}

resource "aws_route_table" "private" {
  #count = var.private ? local.network_resource_count : 0
  count = length(var.private_subnets_ids) == 0 ? var.private ? local.network_resource_count : 0 : 0

  vpc_id = local.vpc_id

  tags = merge(local.tags, {
    Name = "${var.name} private ${count.index}"
  })
}

// workaround for aws eventual consistency API problem
// https://github.com/hashicorp/terraform-provider-aws/issues/13138
resource "null_resource" "wait_routes_private" {
  provisioner "local-exec" {
    command = "sleep 30"
  }

  depends_on = [
    aws_route_table.private,
    aws_internet_gateway.nodes
  ]

}

resource "aws_route" "private-default" {
  depends_on = [
    aws_internet_gateway.nodes,
    aws_route_table.private,
    null_resource.wait_routes_private
  ]

  count = length(var.private_subnets_ids) == 0 ? var.private ? local.network_resource_count : 0 : 0

  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = local.nat_gateway_ids[count.index]
  route_table_id         = local.private_route_tables[count.index]

  timeouts {
    create = "10m"
  }
}

resource "aws_route_table_association" "private" {
  count = var.private ? local.network_resource_count : 0

  route_table_id = local.private_route_tables[count.index]
  subnet_id      = var.private_subnets_ids[count.index]
}

resource "aws_security_group" "cluster" {
  name        = "${var.name}-cluster"
  description = "${var.name} cluster"
  vpc_id      = local.vpc_id

  tags = merge(local.tags, {
    Name = "${var.name}-cluster"
  })
}

resource "null_resource" "network" {
  depends_on = [
    aws_internet_gateway.nodes,
    aws_nat_gateway.private,
    aws_route.private-default,
    aws_route.public-default,
    aws_route_table.private,
    local.public_route_table,
    aws_route_table_association.private,
    aws_route_table_association.public,
    aws_security_group.cluster,
    aws_subnet.private,
    aws_subnet.public,
    aws_vpc.nodes,
  ]

  provisioner "local-exec" {
    when    = destroy
    command = "sleep 300"
  }
}
