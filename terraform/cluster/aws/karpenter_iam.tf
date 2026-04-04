# Karpenter IAM roles and policies
# All resources gated on var.karpenter_enabled

###############################################################################
# Controller IAM Role (IRSA — same OIDC trust pattern as lbc.tf)
###############################################################################

data "aws_iam_policy_document" "assume_karpenter_controller" {
  count = var.karpenter_enabled ? 1 : 0

  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = local.oidc_sub
      values   = ["system:serviceaccount:kube-system:karpenter"]
    }

    principals {
      identifiers = [aws_iam_openid_connect_provider.cluster.arn]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "karpenter_controller" {
  count = var.karpenter_enabled ? 1 : 0

  name               = "${var.name}-karpenter"
  assume_role_policy = data.aws_iam_policy_document.assume_karpenter_controller[0].json
  path               = "/convox/"
  tags               = local.tags
}

# Controller policy: EC2 node lifecycle
# Split into scoped statements per Karpenter v1.x reference policy for least privilege.
# Mutation actions are tag-conditioned to prevent affecting non-Karpenter EC2 resources.
data "aws_iam_policy_document" "karpenter_controller_ec2" {
  count = var.karpenter_enabled ? 1 : 0

  # Read-only describe actions — no conditions needed
  statement {
    sid    = "AllowEC2Describe"
    effect = "Allow"
    actions = [
      "ec2:DescribeAvailabilityZones",
      "ec2:DescribeCapacityReservations",
      "ec2:DescribeImages",
      "ec2:DescribeInstances",
      "ec2:DescribeInstanceTypeOfferings",
      "ec2:DescribeInstanceTypes",
      "ec2:DescribeLaunchTemplates",
      "ec2:DescribeSecurityGroups",
      "ec2:DescribeSpotPriceHistory",
      "ec2:DescribeSubnets",
    ]
    resources = ["*"]
  }

  # RunInstances/CreateFleet — access to shared resources (AMIs, SGs, subnets)
  statement {
    sid    = "AllowEC2RunFromSharedResources"
    effect = "Allow"
    actions = [
      "ec2:RunInstances",
      "ec2:CreateFleet",
    ]
    resources = [
      "arn:${data.aws_partition.current.partition}:ec2:*::image/*",
      "arn:${data.aws_partition.current.partition}:ec2:*::snapshot/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:capacity-reservation/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:security-group/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:subnet/*",
    ]
  }

  # RunInstances/CreateFleet — launch templates must be cluster-owned
  statement {
    sid    = "AllowEC2RunFromOwnedLaunchTemplate"
    effect = "Allow"
    actions = [
      "ec2:RunInstances",
      "ec2:CreateFleet",
    ]
    resources = [
      "arn:${data.aws_partition.current.partition}:ec2:*:*:launch-template/*",
    ]
    condition {
      test     = "StringEquals"
      variable = "aws:ResourceTag/kubernetes.io/cluster/${var.name}"
      values   = ["owned"]
    }
  }

  # RunInstances/CreateFleet/CreateLaunchTemplate — created resources must be tagged with cluster
  statement {
    sid    = "AllowEC2CreateTagged"
    effect = "Allow"
    actions = [
      "ec2:RunInstances",
      "ec2:CreateFleet",
      "ec2:CreateLaunchTemplate",
    ]
    resources = [
      "arn:${data.aws_partition.current.partition}:ec2:*:*:fleet/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:instance/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:volume/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:network-interface/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:launch-template/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:spot-instances-request/*",
    ]
    condition {
      test     = "StringEquals"
      variable = "aws:RequestTag/kubernetes.io/cluster/${var.name}"
      values   = ["owned"]
    }
  }

  # CreateTags — only during resource creation, must include cluster tag
  statement {
    sid    = "AllowEC2CreateTagsOnCreate"
    effect = "Allow"
    actions = [
      "ec2:CreateTags",
    ]
    resources = [
      "arn:${data.aws_partition.current.partition}:ec2:*:*:fleet/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:instance/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:volume/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:network-interface/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:launch-template/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:spot-instances-request/*",
    ]
    condition {
      test     = "StringEquals"
      variable = "aws:RequestTag/kubernetes.io/cluster/${var.name}"
      values   = ["owned"]
    }
    condition {
      test     = "StringEquals"
      variable = "ec2:CreateAction"
      values   = ["RunInstances", "CreateFleet", "CreateLaunchTemplate"]
    }
  }

  # CreateTags on existing cluster-owned instances (e.g. Name, nodeclaim updates)
  statement {
    sid    = "AllowEC2TagExistingInstances"
    effect = "Allow"
    actions = [
      "ec2:CreateTags",
    ]
    resources = [
      "arn:${data.aws_partition.current.partition}:ec2:*:*:instance/*",
    ]
    condition {
      test     = "StringEquals"
      variable = "aws:ResourceTag/kubernetes.io/cluster/${var.name}"
      values   = ["owned"]
    }
    condition {
      test     = "ForAllValues:StringEquals"
      variable = "aws:TagKeys"
      values   = ["eks:eks-cluster-name", "karpenter.sh/nodeclaim", "Name"]
    }
  }

  # TerminateInstances/DeleteLaunchTemplate/DeleteTags — only cluster-owned resources
  statement {
    sid    = "AllowEC2DeleteOwned"
    effect = "Allow"
    actions = [
      "ec2:TerminateInstances",
      "ec2:DeleteLaunchTemplate",
      "ec2:DeleteTags",
    ]
    resources = [
      "arn:${data.aws_partition.current.partition}:ec2:*:*:instance/*",
      "arn:${data.aws_partition.current.partition}:ec2:*:*:launch-template/*",
    ]
    condition {
      test     = "StringEquals"
      variable = "aws:ResourceTag/kubernetes.io/cluster/${var.name}"
      values   = ["owned"]
    }
  }

  # PassRole — scoped to Karpenter node role only, restricted to EC2 service
  statement {
    sid    = "AllowPassingRoleToEC2"
    effect = "Allow"
    actions = [
      "iam:PassRole",
    ]
    resources = [aws_iam_role.karpenter_nodes[0].arn]
    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "karpenter_controller_ec2" {
  count = var.karpenter_enabled ? 1 : 0

  name   = "ec2-node-lifecycle"
  role   = aws_iam_role.karpenter_controller[0].name
  policy = data.aws_iam_policy_document.karpenter_controller_ec2[0].json
}

# Controller policy: IAM instance profile management (Karpenter v1.9.0+ manages profiles itself)
data "aws_iam_policy_document" "karpenter_controller_iam" {
  count = var.karpenter_enabled ? 1 : 0

  statement {
    sid    = "AllowInstanceProfileManagement"
    effect = "Allow"
    actions = [
      "iam:AddRoleToInstanceProfile",
      "iam:CreateInstanceProfile",
      "iam:DeleteInstanceProfile",
      "iam:RemoveRoleFromInstanceProfile",
      "iam:TagInstanceProfile",
    ]
    resources = ["*"]
    condition {
      test     = "StringEquals"
      variable = "aws:RequestTag/kubernetes.io/cluster/${var.name}"
      values   = ["owned"]
    }
  }

  statement {
    sid    = "AllowInstanceProfileManagementByTag"
    effect = "Allow"
    actions = [
      "iam:AddRoleToInstanceProfile",
      "iam:DeleteInstanceProfile",
      "iam:RemoveRoleFromInstanceProfile",
    ]
    resources = ["*"]
    condition {
      test     = "StringEquals"
      variable = "aws:ResourceTag/kubernetes.io/cluster/${var.name}"
      values   = ["owned"]
    }
  }

  # Read-only instance profile discovery — no tag conditions (GetInstanceProfile
  # has no request tags and the profile may not yet be tagged when first queried)
  statement {
    sid    = "AllowInstanceProfileRead"
    effect = "Allow"
    actions = [
      "iam:GetInstanceProfile",
      "iam:ListInstanceProfiles",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "karpenter_controller_iam" {
  count = var.karpenter_enabled ? 1 : 0

  name   = "iam-instance-profile"
  role   = aws_iam_role.karpenter_controller[0].name
  policy = data.aws_iam_policy_document.karpenter_controller_iam[0].json
}

# Controller policy: EKS cluster discovery
data "aws_iam_policy_document" "karpenter_controller_eks" {
  count = var.karpenter_enabled ? 1 : 0

  statement {
    sid    = "AllowEKSClusterDiscovery"
    effect = "Allow"
    actions = [
      "eks:DescribeCluster",
    ]
    resources = [aws_eks_cluster.cluster.arn]
  }
}

resource "aws_iam_role_policy" "karpenter_controller_eks" {
  count = var.karpenter_enabled ? 1 : 0

  name   = "eks-cluster-discovery"
  role   = aws_iam_role.karpenter_controller[0].name
  policy = data.aws_iam_policy_document.karpenter_controller_eks[0].json
}

# Controller policy: SQS interruption queue
data "aws_iam_policy_document" "karpenter_controller_sqs" {
  count = var.karpenter_enabled ? 1 : 0

  statement {
    sid    = "AllowSQSInterruption"
    effect = "Allow"
    actions = [
      "sqs:DeleteMessage",
      "sqs:GetQueueAttributes",
      "sqs:GetQueueUrl",
      "sqs:ReceiveMessage",
    ]
    resources = [aws_sqs_queue.karpenter_interruption[0].arn]
  }
}

resource "aws_iam_role_policy" "karpenter_controller_sqs" {
  count = var.karpenter_enabled ? 1 : 0

  name   = "sqs-interruption"
  role   = aws_iam_role.karpenter_controller[0].name
  policy = data.aws_iam_policy_document.karpenter_controller_sqs[0].json
}

# Controller policy: Pricing and SSM for instance type discovery
data "aws_iam_policy_document" "karpenter_controller_pricing" {
  count = var.karpenter_enabled ? 1 : 0

  statement {
    sid    = "AllowPricingAndSSM"
    effect = "Allow"
    actions = [
      "pricing:GetProducts",
      "ssm:GetParameter",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "karpenter_controller_pricing" {
  count = var.karpenter_enabled ? 1 : 0

  name   = "pricing-discovery"
  role   = aws_iam_role.karpenter_controller[0].name
  policy = data.aws_iam_policy_document.karpenter_controller_pricing[0].json
}

###############################################################################
# Node IAM Role (separate from existing nodes role to avoid privilege creep)
###############################################################################

resource "aws_iam_role" "karpenter_nodes" {
  count = var.karpenter_enabled ? 1 : 0

  name               = "${var.name}-karpenter-nodes"
  assume_role_policy = data.aws_iam_policy_document.assume_ec2.json
  path               = "/convox/"
  tags               = local.tags
}

resource "aws_iam_role_policy_attachment" "karpenter_nodes_worker" {
  count = var.karpenter_enabled ? 1 : 0

  role       = aws_iam_role.karpenter_nodes[0].name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKSWorkerNodePolicy"
}

resource "aws_iam_role_policy_attachment" "karpenter_nodes_cni" {
  count = var.karpenter_enabled ? 1 : 0

  role       = aws_iam_role.karpenter_nodes[0].name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEKS_CNI_Policy"
}

resource "aws_iam_role_policy_attachment" "karpenter_nodes_ecr" {
  count = var.karpenter_enabled ? 1 : 0

  role       = aws_iam_role.karpenter_nodes[0].name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy_attachment" "karpenter_nodes_ssm" {
  count = var.karpenter_enabled ? 1 : 0

  role       = aws_iam_role.karpenter_nodes[0].name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy_attachment" "karpenter_nodes_ebs" {
  count = var.karpenter_enabled ? 1 : 0

  role       = aws_iam_role.karpenter_nodes[0].name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
}

###############################################################################
# EKS Access Entry (allows Karpenter-managed nodes to join the cluster)
###############################################################################

resource "aws_eks_access_entry" "karpenter_nodes" {
  count = var.karpenter_enabled ? 1 : 0

  depends_on = [null_resource.karpenter_access_config]

  cluster_name  = aws_eks_cluster.cluster.name
  principal_arn = aws_iam_role.karpenter_nodes[0].arn
  type          = "EC2_LINUX"
  tags          = local.tags
}
