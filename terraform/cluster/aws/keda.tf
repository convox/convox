# KEDA deployment for AWS EKS via Terraform
# This file creates KEDA resources if var.keda_enable is true

locals {
  keda_sa        = "keda-operator"
  keda_namespace = "keda"
  keda_labels = {
    "app.kubernetes.io/name" = "keda-operator"
  }
}


data "aws_iam_policy_document" "assume_keda" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = local.oidc_sub
      values   = ["system:serviceaccount:${local.keda_namespace}:${local.keda_sa}"]
    }

    principals {
      identifiers = [aws_iam_openid_connect_provider.cluster.arn]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "keda" {
  count              = var.keda_enable ? 1 : 0
  name               = "${var.name}-keda"
  assume_role_policy = data.aws_iam_policy_document.assume_keda.json
  path               = "/convox/"
  tags               = local.tags
}


data "aws_iam_policy_document" "keda_aws_scalar" {
  statement {
    effect = "Allow"
    actions = [
      "cloudwatch:GetMetricData",
      "sqs:GetQueueAttributes",
      "sqs:GetQueueUrl",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "keda_aws_scalar" {
  count  = var.keda_enable ? 1 : 0
  name   = "keda-aws-scalar"
  role   = aws_iam_role.keda[0].name
  policy = data.aws_iam_policy_document.keda_aws_scalar.json
}

resource "helm_release" "keda" {
  count      = var.keda_enable ? 1 : 0
  name       = "keda"
  repository = "https://kedacore.github.io/charts"
  chart      = "keda"
  version    = "2.18.3"
  namespace  = local.keda_namespace

  create_namespace = true

  values = [
    yamlencode({
      serviceAccount = {
        create = true
        name   = local.keda_sa
      }
      podAnnotations = {
        keda = {
          "traffic.sidecar.istio.io/excludeInboundPorts"  = "9666"
          "traffic.sidecar.istio.io/excludeOutboundPorts" = "9443,6443"
        }

        metricsAdapter = {
          "traffic.sidecar.istio.io/excludeInboundPorts"  = "6443"
          "traffic.sidecar.istio.io/excludeOutboundPorts" = "9666,9443"
        }

        webhooks = {
          "traffic.sidecar.istio.io/excludeInboundPorts"  = "9443"
          "traffic.sidecar.istio.io/excludeOutboundPorts" = "9666,6443"
        }
      }

      podIdentity = {
        aws = {
          irsa = {
            enabled = true
            roleArn = aws_iam_role.keda[0].arn
          }
        }
      }
    })
  ]
}
