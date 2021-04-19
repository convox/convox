provider "kubernetes" {
  cluster_ca_certificate = base64decode(aws_eks_cluster.cluster.certificate_authority.0.data)
  host                   = aws_eks_cluster.cluster.endpoint
  token                  = data.aws_eks_cluster_auth.cluster.token

  load_config_file = false
}

locals {
  oidc_sub           = "${replace(aws_iam_openid_connect_provider.cluster.url, "https://", "")}:sub"
  gpu_type           = substr(var.node_type, 0, 1) == "g" || substr(var.node_type, 0, 1) == "p"
  arm_type           = substr(var.node_type, 0, 2) == "a1" || substr(var.node_type, 0, 3) == "c6g" || substr(var.node_type, 0, 3) == "m6g" || substr(var.node_type, 0, 3) == "r6g"
  availability_zones = var.availability_zones != "" ? compact(split(",", var.availability_zones)) : data.aws_availability_zones.available.names
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
    role_arn  = replace(aws_iam_role.nodes.arn, "role/convox/", "role/") # eks barfs on roles with paths
  }
}

resource "aws_eks_node_group" "cluster" {
  depends_on = [
    aws_eks_cluster.cluster,
    aws_iam_openid_connect_provider.cluster,
  ]

  count = 3

  ami_type        = local.gpu_type ? "AL2_x86_64_GPU" : local.arm_type ? "AL2_ARM_64" : "AL2_x86_64"
  cluster_name    = aws_eks_cluster.cluster.name
  disk_size       = random_id.node_group.keepers.node_disk
  instance_types  = [random_id.node_group.keepers.node_type]
  node_group_name = "${var.name}-${local.availability_zones[count.index]}-${random_id.node_group.hex}"
  node_role_arn   = random_id.node_group.keepers.role_arn
  subnet_ids      = [var.private ? aws_subnet.private[count.index].id : aws_subnet.public[count.index].id]

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

resource "kubernetes_storage_class" "volumes_storage_class" {
  metadata {
    name = "convox-volumes-storage-class"
    annotations = {
      "storageclass.kubernetes.io/is-default-class" = "true"
    }
  }
  storage_provisioner = "efs.csi.aws.com"
  reclaim_policy      = "Retain"

  depends_on = [
    aws_eks_node_group.cluster,
  ]
}

