locals {
  arn_prefix = "${substr(data.aws_region.current.name, 0, 6)  == "us-gov" ? "aws-us-gov" : "aws"}"
}

data "aws_iam_policy_document" "assume_fluentd" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = var.oidc_sub
      values   = ["system:serviceaccount:${var.namespace}:fluentd"]
    }

    principals {
      identifiers = [var.oidc_arn]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "fluentd" {
  name               = "${var.rack}-fluentd"
  assume_role_policy = data.aws_iam_policy_document.assume_fluentd.json
  path               = "/convox/"
  tags               = local.tags
}

data "aws_iam_policy_document" "fluentd" {
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:DescribeLogGroups",
    ]
    resources = [
      "arn:${local.arn_prefix}:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:*"
    ]
  }

  statement {
    actions = [
      "logs:CreateLogStream",
      "logs:DescribeLogStreams",
      "logs:PutLogEvents",
    ]
    resources = [
      "arn:${local.arn_prefix}:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:/convox/*"
    ]
  }
}

resource "aws_iam_role_policy" "fluentd" {
  name   = "fluentd"
  role   = aws_iam_role.fluentd.name
  policy = data.aws_iam_policy_document.fluentd.json
}
