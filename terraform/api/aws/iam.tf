data "aws_partition" "current" {}

data "aws_iam_policy_document" "assume_api" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = var.oidc_sub
      values   = ["system:serviceaccount:${var.namespace}:api"]
    }

    principals {
      identifiers = [var.oidc_arn]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "api" {
  name               = "${var.name}-api"
  assume_role_policy = data.aws_iam_policy_document.assume_api.json
  path               = "/convox/"
  tags               = local.tags
}

data "aws_iam_policy_document" "iam_role_manage" {
  statement {
    effect = "Allow"
    actions = [
      "iam:CreateRole",
      "iam:UpdateRole",
      "iam:DeleteRole",
      "iam:PassRole",
      "iam:TagRole",
      "iam:UntagRole",
      "iam:GetRole",
      "iam:GetRolePolicy",
      "iam:ListRolePolicies",
      "iam:ListAttachedRolePolicies",
      "iam:AttachRolePolicy",
      "iam:DetachRolePolicy",
      "iam:ListPolicyVersions",
      "iam:UpdateAssumeRolePolicy",
      "iam:UpdateRoleDescription",
    ]
    resources = [
      "arn:${data.aws_partition.current.partition}:iam::${data.aws_caller_identity.current.account_id}:role/convox/*",
    ]
  }
}

data "aws_iam_policy_document" "eks_pod_identitiy" {
  statement {
    effect = "Allow"
    actions = [
      "eks:ListPodIdentityAssociations",
      "eks:CreatePodIdentityAssociation",
      "eks:DescribePodIdentityAssociation",
      "eks:DeletePodIdentityAssociation",
      "eks:UpdatePodIdentityAssociation",
    ]
    resources = ["*"]
  }
}

data "aws_iam_policy_document" "ec2_key_pair" {
  statement {
    actions   = ["ec2:CreateKeyPair*"]
    resources = ["*"]
  }
}

data "aws_iam_policy_document" "logs" {
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:DescribeLogStreams",
      "logs:FilterLogEvents",
      "logs:PutLogEvents",
      "logs:PutRetentionPolicy",
      "logs:DeleteRetentionPolicy",
    ]
    resources = [
      "arn:${data.aws_partition.current.partition}:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:${var.name}-*",
      "arn:${data.aws_partition.current.partition}:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:/convox/${var.name}/*",
    ]
  }
}

data "aws_iam_policy_document" "storage" {
  statement {
    actions = [
      "s3:ListBucket",
    ]
    resources = [
      var.custom_provided_bucket != "" ? data.aws_s3_bucket.custom_bucket[0].arn : aws_s3_bucket.storage.arn,
    ]
  }

  statement {
    actions = [
      "s3:DeleteObject",
      "s3:HeadObject",
      "s3:GetObject",
      "s3:ListObjects",
      "s3:PutObject",
    ]
    resources = [
      var.custom_provided_bucket != "" ? "${data.aws_s3_bucket.custom_bucket[0].arn}/*" : "${aws_s3_bucket.storage.arn}/*",
    ]
  }
}

data "aws_iam_policy_document" "rds_provisioner" {
  statement {
    effect = "Allow"
    actions = [
      "ec2:CreateSecurityGroup",
      "ec2:DeleteSecurityGroup",
      "ec2:Describe*",
      "ec2:AuthorizeSecurityGroupIngress",
      "ec2:AuthorizeSecurityGroupEgress",
      "ec2:RevokeSecurityGroupIngress",
      "ec2:RevokeSecurityGroupEgress",
      "ec2:ModifySecurityGroupRules",
      "ec2:CreateTags",
      "ec2:DescribeInstanceTypes",
    ]
    resources = ["*"]
  }

  statement {
    effect = "Allow"
    actions = [
      "rds:CreateDBInstance*",
      "rds:DeleteDBInstance*",
      "rds:ModifyDBInstance*",
      "rds:CreateDBSnapshot",
      "rds:Describe*",
      "rds:CreateDBSubnetGroup",
      "rds:DeleteDBSubnetGroup",
      "rds:ModifyDBSubnetGroup",
      "rds:AddTagsToResource",
      "rds:RestoreDBInstanceFromDBSnapshot",
      "rds:PromoteReadReplica",
    ]
    resources = ["*"]
  }

  statement {
    effect = "Allow"
    actions = [
      "elasticache:Create*",
      "elasticache:Modify*",
      "elasticache:Describe*",
      "elasticache:Delete*",
      "elasticache:List*",
      "elasticache:IncreaseReplicaCount",
      "elasticache:ListTagsForResource",
      "elasticache:AddTagsToResource",
      "elasticache:DecreaseReplicaCount",
    ]
    resources = ["*"]
  }

  statement {
    effect = "Allow"
    actions = ["iam:CreateServiceLinkedRole"]
    resources = ["arn:${data.aws_partition.current.partition}:iam::*:role/aws-service-role/rds.amazonaws.com/AWSServiceRoleForRDS"]
    condition {
      test     = "StringLike"
      variable = "iam:AWSServiceName"
      values   = ["rds.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy_attachment" "api_ecr" {
  role       = aws_iam_role.api.name
  policy_arn = "arn:${data.aws_partition.current.partition}:iam::aws:policy/AmazonEC2ContainerRegistryFullAccess"
}

resource "aws_iam_role_policy" "api_ec2_key_pair" {
  name   = "ec2_key_pair"
  role   = aws_iam_role.api.name
  policy = data.aws_iam_policy_document.ec2_key_pair.json
}

resource "aws_iam_role_policy" "api_logs" {
  name   = "logs"
  role   = aws_iam_role.api.name
  policy = data.aws_iam_policy_document.logs.json
}

resource "aws_iam_role_policy" "api_storage" {
  name   = "storage"
  role   = aws_iam_role.api.name
  policy = data.aws_iam_policy_document.storage.json
}

resource "aws_iam_role_policy" "api_iam_manage" {
  name   = "api-iam-manage"
  role   = aws_iam_role.api.name
  policy = data.aws_iam_policy_document.iam_role_manage.json
}

resource "aws_iam_role_policy" "api_eks_pod_identity" {
  name   = "api-eks-pod-identity"
  role   = aws_iam_role.api.name
  policy = data.aws_iam_policy_document.eks_pod_identitiy.json
}

resource "aws_iam_role_policy" "rds_provisioner" {
  name   = "api-rds-provisioner"
  role   = aws_iam_role.api.name
  policy = data.aws_iam_policy_document.rds_provisioner.json
}

data "aws_iam_policy_document" "assume_cert_manager" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = var.oidc_sub
      values   = ["system:serviceaccount:cert-manager:cert-manager"]
    }

    principals {
      identifiers = [var.oidc_arn]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "cert-manager" {
  name               = "${var.name}-cert-manager"
  assume_role_policy = data.aws_iam_policy_document.assume_cert_manager.json
  path               = "/convox/"
  tags               = local.tags
}
