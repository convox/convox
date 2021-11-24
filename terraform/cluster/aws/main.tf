provider "kubernetes" {
  cluster_ca_certificate = base64decode(aws_eks_cluster.cluster.certificate_authority.0.data)
  host                   = aws_eks_cluster.cluster.endpoint

  load_config_file = false
  exec {
    api_version = "client.authentication.k8s.io/v1alpha1"
    args        = ["eks", "get-token", "--cluster-name", var.name]
    command     = "aws"
  }
}

locals {
  availability_zones     = var.availability_zones != "" ? compact(split(",", var.availability_zones)) : data.aws_availability_zones.available.names
  network_resource_count = var.high_availability ? 3 : 2
  oidc_sub               = "${replace(aws_iam_openid_connect_provider.cluster.url, "https://", "")}:sub"
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

// the cluster API takes some seconds to be available even when aws reports that the cluster is ready
// https://github.com/terraform-aws-modules/terraform-aws-eks/issues/621
resource "null_resource" "wait_k8s_api" {
  provisioner "local-exec" {
    command = "sleep 60"
  }

  depends_on = [
    aws_eks_cluster.cluster
  ]

}

resource "aws_eks_cluster" "cluster" {
  depends_on = [
    null_resource.delay_cluster,
    null_resource.iam,
    null_resource.network,
  ]

  name     = var.name
  role_arn = aws_iam_role.cluster.arn
  version  = var.k8s_version

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
    role_arn  = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  }
}

resource "aws_eks_node_group" "cluster" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  count = var.high_availability ? 3 : 1

  ami_type        = var.gpu_type ? "AL2_x86_64_GPU" : var.arm_type ? "AL2_ARM_64" : "AL2_x86_64"
  cluster_name    = aws_eks_cluster.cluster.name
  disk_size       = random_id.node_group.keepers.node_disk
  instance_types  = [random_id.node_group.keepers.node_type]
  node_group_name = "${var.name}-${local.availability_zones[count.index]}-${random_id.node_group.hex}"
  node_role_arn   = random_id.node_group.keepers.role_arn
  subnet_ids      = [var.private ? aws_subnet.private[count.index].id : aws_subnet.public[count.index].id]
  version         = var.k8s_version

  scaling_config {
    desired_size = 1
    min_size     = 1
    max_size     = var.high_availability ? 100 : 3
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
