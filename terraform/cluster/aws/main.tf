terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.22"
}

provider "local" {
  version = "~> 1.3"
}

provider "null" {
  version = "~> 2.1"
}

locals {
  oidc_sub = "${replace(aws_iam_openid_connect_provider.cluster.url, "https://", "")}:sub"
}

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
    aws_iam_role_policy_attachment.cluster_eks_cluster,
    aws_iam_role_policy_attachment.cluster_eks_service,
    aws_subnet.private,
    aws_subnet.public,
    null_resource.delay_cluster,
  ]

  name     = var.name
  role_arn = aws_iam_role.cluster.arn

  vpc_config {
    endpoint_public_access  = true
    endpoint_private_access = false
    security_group_ids      = [aws_security_group.cluster.id]
    subnet_ids              = concat(aws_subnet.public.*.id)
  }
}

resource "aws_eks_node_group" "cluster" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
    aws_iam_role_policy_attachment.cluster_eks_cluster,
    aws_iam_role_policy_attachment.cluster_eks_service,
    aws_iam_role_policy_attachment.nodes_ecr,
    aws_iam_role_policy_attachment.nodes_eks_cni,
    aws_iam_role_policy_attachment.nodes_eks_worker,
    aws_route.private-default,
    aws_route.public-default,
    aws_route_table.private,
    aws_route_table.public,
    aws_route_table_association.private,
    aws_route_table_association.public,
  ]

  count = 3

  cluster_name    = aws_eks_cluster.cluster.name
  disk_size       = var.node_disk
  instance_types  = [var.node_type]
  node_group_name = "${var.name}-${data.aws_availability_zones.available.names[count.index]}"
  node_role_arn   = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  subnet_ids      = [aws_subnet.private[count.index].id]

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
