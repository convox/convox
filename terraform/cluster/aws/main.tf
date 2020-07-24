terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.49"
}

provider "kubernetes" {
  version = "~> 1.11"

  cluster_ca_certificate = base64decode(aws_eks_cluster.cluster.certificate_authority.0.data)
  host                   = aws_eks_cluster.cluster.endpoint
  token                  = data.aws_eks_cluster_auth.cluster.token

  load_config_file = false
}

provider "local" {
  version = "~> 1.3"
}

provider "null" {
  version = "~> 2.1"
}

locals {
  availability_zones = slice(data.aws_availability_zones.available.names, 0, 2)
  oidc_sub           = "${replace(aws_iam_openid_connect_provider.cluster.url, "https://", "")}:sub"
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_eks_cluster_auth" "cluster" {
  name = aws_eks_cluster.cluster.id
}

data "aws_region" "current" {}

resource "null_resource" "delay_cluster" {
  provisioner "local-exec" {
    command = "sleep 15"
  }

  triggers = {
    "eks_cluster" = aws_iam_role_policy_attachment.cluster_eks_cluster.id,
    "eks_service" = aws_iam_role_policy_attachment.cluster_eks_service.id,
  }
}

resource "aws_eks_cluster" "cluster" {
  depends_on = [
    null_resource.delay_cluster,
    null_resource.iam,
    null_resource.network,
  ]

  name     = var.name
  role_arn = aws_iam_role.cluster.arn
  version  = "1.17"

  vpc_config {
    endpoint_public_access  = true
    endpoint_private_access = false
    security_group_ids      = [aws_security_group.cluster.id]
    subnet_ids              = concat(aws_subnet.public.*.id)
  }
}

resource "aws_iam_openid_connect_provider" "cluster" {
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["9e99a48a9960b14926bb7f3b02e22da2b0ab7280"]
  url             = aws_eks_cluster.cluster.identity.0.oidc.0.issuer
}

resource "random_id" "node_group" {
  byte_length = 8

  keepers = {
    node_disk = var.node_disk
    node_type = var.node_type
    private   = var.private
  }
}

# the node group section is copy-pasted to allow for rolling restarts. when using count or for_each
# the dependency chain does not allow them to begin their destruction at different times

resource "aws_eks_node_group" "cluster" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  cluster_name    = aws_eks_cluster.cluster.name
  disk_size       = random_id.node_group.keepers.node_disk
  instance_types  = [random_id.node_group.keepers.node_type]
  node_group_name = "${var.name}-${data.aws_availability_zones.available.names[0]}-${random_id.node_group.hex}"
  node_role_arn   = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  subnet_ids      = [var.private ? aws_subnet.private[0].id : aws_subnet.public[0].id]

  scaling_config {
    desired_size = 1
    min_size     = 1
    max_size     = 100
  }

  lifecycle {
    create_before_destroy = true
    ignore_changes        = [scaling_config[0].desired_size]
  }
}

resource "aws_eks_node_group" "cluster1" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_eks_node_group.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  cluster_name    = aws_eks_cluster.cluster.name
  disk_size       = random_id.node_group.keepers.node_disk
  instance_types  = [random_id.node_group.keepers.node_type]
  node_group_name = "${var.name}-${data.aws_availability_zones.available.names[1]}-${random_id.node_group.hex}"
  node_role_arn   = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  subnet_ids      = [var.private ? aws_subnet.private[1].id : aws_subnet.public[1].id]

  scaling_config {
    desired_size = 1
    min_size     = 1
    max_size     = 100
  }

  lifecycle {
    create_before_destroy = true
    ignore_changes        = [scaling_config[0].desired_size]
  }
}

resource "aws_eks_node_group" "cluster2" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_eks_node_group.cluster1,
    aws_iam_openid_connect_provider.cluster,
  ]

  cluster_name    = aws_eks_cluster.cluster.name
  disk_size       = random_id.node_group.keepers.node_disk
  instance_types  = [random_id.node_group.keepers.node_type]
  node_group_name = "${var.name}-${data.aws_availability_zones.available.names[2]}-${random_id.node_group.hex}"
  node_role_arn   = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  subnet_ids      = [var.private ? aws_subnet.private[2].id : aws_subnet.public[2].id]

  scaling_config {
    desired_size = 1
    min_size     = 1
    max_size     = 100
  }

  lifecycle {
    create_before_destroy = true
    ignore_changes        = [scaling_config[0].desired_size]
  }
}

resource "local_file" "kubeconfig" {
  depends_on = [
    aws_eks_node_group.cluster,
  ]

  filename = pathexpand("~/.kube/config.aws.${var.name}")
  content = templatefile("${path.module}/kubeconfig.tpl", {
    ca       = aws_eks_cluster.cluster.certificate_authority.0.data
    cluster  = aws_eks_cluster.cluster.id
    endpoint = aws_eks_cluster.cluster.endpoint
  })
}
