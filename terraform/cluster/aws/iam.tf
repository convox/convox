data "aws_partition" "current" {}

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
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSClusterPolicy"
}

resource "aws_iam_role_policy_attachment" "cluster_eks_service" {
  depends_on = [
    aws_iam_role_policy_attachment.cluster_eks_cluster
  ]
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSServicePolicy"
}

resource "aws_iam_role_policy_attachment" "cluster_ec2_readonly" {
  depends_on = [
    aws_iam_role_policy_attachment.cluster_eks_cluster,
    aws_iam_role_policy_attachment.cluster_eks_service
  ]
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEC2ReadOnlyAccess"
}

resource "aws_iam_role" "nodes" {
  assume_role_policy = data.aws_iam_policy_document.assume_ec2.json
  name               = "${var.name}-nodes"
  path               = "/convox/"
}

data "aws_iam_policy_document" "eks_pod_identitiy" {
  statement {
    effect = "Allow"
    actions = [
      "eks-auth:AssumeRoleForPodIdentity",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "nodes_eks_pod_identitiy" {
  name   = "eks-pod-identitiy"
  role   = aws_iam_role.nodes.name
  policy = data.aws_iam_policy_document.eks_pod_identitiy.json
}

resource "aws_iam_role_policy_attachment" "nodes_ecr" {
  role       = aws_iam_role.nodes.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy_attachment" "nodes_eks_cni" {
  depends_on = [
    aws_iam_role_policy_attachment.nodes_ecr
  ]
  role       = aws_iam_role.nodes.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKS_CNI_Policy"
}

resource "aws_iam_role_policy_attachment" "nodes_eks_worker" {
  depends_on = [
    aws_iam_role_policy_attachment.nodes_ecr,
    aws_iam_role_policy_attachment.nodes_eks_cni
  ]
  role       = aws_iam_role.nodes.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSWorkerNodePolicy"
}


resource "null_resource" "iam" {
  depends_on = [
    aws_iam_role_policy_attachment.cluster_ec2_readonly,
    aws_iam_role_policy_attachment.cluster_eks_cluster,
    aws_iam_role_policy_attachment.cluster_eks_service,
    aws_iam_role_policy_attachment.nodes_ecr,
    aws_iam_role_policy_attachment.nodes_eks_cni,
    aws_iam_role_policy_attachment.nodes_eks_worker,
  ]

  provisioner "local-exec" {
    when    = destroy
    command = "sleep 300"
  }
}
