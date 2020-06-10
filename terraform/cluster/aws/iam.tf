locals {
  arn_prefix = "${substr(data.aws_region.current.name, 0, 6)  == "us-gov" ? "aws-us-gov" : "aws"}"
}

resource "aws_iam_openid_connect_provider" "cluster" {
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["9e99a48a9960b14926bb7f3b02e22da2b0ab7280"]
  url             = aws_eks_cluster.cluster.identity.0.oidc.0.issuer
}

data "aws_iam_policy_document" "assume_ec2" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "assume_eks" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "cluster" {
  assume_role_policy = data.aws_iam_policy_document.assume_eks.json
  name               = "${var.name}-cluster"
  path               = "/convox/"
}

resource "aws_iam_role_policy_attachment" "cluster_eks_cluster" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:${local.arn_prefix}:iam::aws:policy/AmazonEKSClusterPolicy"
}

resource "aws_iam_role_policy_attachment" "cluster_eks_service" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:${local.arn_prefix}:iam::aws:policy/AmazonEKSServicePolicy"
}

resource "aws_iam_role_policy_attachment" "cluster_ec2_readonly" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:${local.arn_prefix}:iam::aws:policy/AmazonEC2ReadOnlyAccess"
}

resource "aws_iam_role" "nodes" {
  assume_role_policy = data.aws_iam_policy_document.assume_ec2.json
  name               = "${var.name}-nodes"
  path               = "/convox/"
}

resource "aws_iam_role_policy_attachment" "nodes_ecr" {
  role       = aws_iam_role.nodes.name
  policy_arn = "arn:${local.arn_prefix}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy_attachment" "nodes_eks_cni" {
  role       = aws_iam_role.nodes.name
  policy_arn = "arn:${local.arn_prefix}:iam::aws:policy/AmazonEKS_CNI_Policy"
}

resource "aws_iam_role_policy_attachment" "nodes_eks_worker" {
  role       = aws_iam_role.nodes.name
  policy_arn = "arn:${local.arn_prefix}:iam::aws:policy/AmazonEKSWorkerNodePolicy"
}
