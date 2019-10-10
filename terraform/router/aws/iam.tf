data "aws_iam_policy_document" "nodes-assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "AWS"
      identifiers = [var.nodes_role]
    }
  }
}

resource "aws_iam_role" "router" {
  name               = "${var.name}-router"
  assume_role_policy = data.aws_iam_policy_document.nodes-assume.json
  path               = "/convox/"
  tags               = local.tags
}

data "aws_iam_policy_document" "router" {
  statement {
    resources = [aws_dynamodb_table.cache.arn]
    actions = [
      "dynamodb:DeleteItem",
      "dynamodb:GetItem",
      "dynamodb:PutItem",
    ]
  }

  statement {
    resources = [aws_dynamodb_table.hosts.arn]
    actions = [
      "dynamodb:GetItem",
      "dynamodb:UpdateItem",
    ]
  }

  statement {
    resources = [aws_dynamodb_table.targets.arn]
    actions = [
      "dynamodb:GetItem",
      "dynamodb:UpdateItem",
    ]
  }
}

resource "aws_iam_role_policy" "router" {
  name   = "${var.name}-router"
  role   = aws_iam_role.router.id
  policy = data.aws_iam_policy_document.router.json
}
