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
      aws_s3_bucket.storage.arn,
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
      "${aws_s3_bucket.storage.arn}/*",
    ]
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
