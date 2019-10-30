terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.22"
}

provider "local" {
  version = "~> 1.3"
}

data "aws_caller_identity" "current" {
}

data "aws_ami" "node" {
  most_recent = true
  owners      = ["602401143452"] # aws eks team

  filter {
    name   = "name"
    values = ["amazon-eks-node-${var.kubernetes_version}-v*"]
  }
}

resource "aws_cloudformation_stack" "nodes" {
  depends_on = [aws_internet_gateway.nodes]

  capabilities  = ["CAPABILITY_IAM"]
  on_failure    = "DELETE"
  name          = "${var.name}-nodes"
  template_body = file("${path.module}/cloudformation.yml")

  parameters = {
    Ami      = data.aws_ami.node.id
    Cluster  = aws_eks_cluster.cluster.id
    Role     = aws_iam_role.nodes.name
    Security = aws_security_group.nodes.id
    SshKey   = var.ssh_key
    Subnets  = join(",", aws_subnet.private.*.id)
    Type     = var.node_type
    Vpc      = aws_vpc.nodes.id
  }
}

resource "aws_eks_cluster" "cluster" {
  name     = var.name
  role_arn = aws_iam_role.cluster.arn

  vpc_config {
    endpoint_public_access  = true
    endpoint_private_access = false
    security_group_ids      = [aws_security_group.cluster.id]
    subnet_ids              = concat(aws_subnet.public.*.id)
  }
}

resource "null_resource" "after_cluster" {
  provisioner "local-exec" {
    command = "sleep 30"
  }
}

resource "local_file" "kubeconfig" {
  depends_on = [
    aws_cloudformation_stack.nodes,
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
    aws_security_group_rule.cluster_egress_control,
    aws_security_group_rule.cluster_egress_traffic,
    aws_security_group_rule.cluster_ingress_control,
    aws_security_group_rule.nodes_egress_internet,
    aws_security_group_rule.nodes_ingress_control,
    aws_security_group_rule.nodes_ingress_internal,
    aws_security_group_rule.nodes_ingress_mtu,
    aws_security_group_rule.nodes_ingress_traffic,
    null_resource.after_cluster,
  ]

  filename = pathexpand("~/.kube/config.${var.name}")
  content = templatefile("${path.module}/kubeconfig.tpl", {
    ca       = aws_eks_cluster.cluster.certificate_authority.0.data
    cluster  = aws_eks_cluster.cluster.id
    endpoint = aws_eks_cluster.cluster.endpoint
  })
}

provider "kubernetes" {
  version = "~> 1.8"

  alias = "direct"

  load_config_file       = false
  cluster_ca_certificate = base64decode(aws_eks_cluster.cluster.certificate_authority.0.data)
  host                   = aws_eks_cluster.cluster.endpoint
  exec {
    api_version = "client.authentication.k8s.io/v1alpha1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", aws_eks_cluster.cluster.id]
  }
}

resource "kubernetes_config_map" "auth" {
  depends_on = [
    aws_cloudformation_stack.nodes,
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
    aws_security_group_rule.cluster_egress_control,
    aws_security_group_rule.cluster_egress_traffic,
    aws_security_group_rule.cluster_ingress_control,
    aws_security_group_rule.nodes_egress_internet,
    aws_security_group_rule.nodes_ingress_control,
    aws_security_group_rule.nodes_ingress_internal,
    aws_security_group_rule.nodes_ingress_mtu,
    aws_security_group_rule.nodes_ingress_traffic,
    null_resource.after_cluster,
  ]

  provider = kubernetes.direct

  metadata {
    namespace = "kube-system"
    name      = "aws-auth"
  }

  data = {
    mapRoles = <<EOF
      - rolearn: "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/${aws_iam_role.nodes.name}"
        username: system:node:{{EC2PrivateDNSName}}
        groups:
          - system:bootstrappers
          - system:nodes
    EOF
  }
}
