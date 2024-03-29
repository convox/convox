locals {
  lbc_sa = "aws-lbc"
  lbc_labels = {
    "eks-addon" : "aws-load-balancer-controller",
    "k8s-app" : "aws-lbc",
  }
}

data "aws_iam_policy_document" "assume_lbc" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    condition {
      test     = "StringEquals"
      variable = local.oidc_sub
      values   = ["system:serviceaccount:kube-system:${local.lbc_sa}"]
    }

    principals {
      identifiers = [aws_iam_openid_connect_provider.cluster.arn]
      type        = "Federated"
    }
  }
}

resource "aws_iam_role" "lbc" {
  name               = "${var.name}-lbc"
  assume_role_policy = data.aws_iam_policy_document.assume_lbc.json
  path               = "/convox/"
  tags               = local.tags
}

resource "aws_iam_role_policy" "lbc_policy" {
  name   = "aws-lbc"
  role   = aws_iam_role.lbc.name
  policy = file("${path.module}/files/lbc_policy.json")
}

resource "kubernetes_service_account" "lbc" {
  depends_on = [
    null_resource.wait_eks_addons,
    aws_iam_role.lbc,
    aws_iam_role_policy.lbc_policy
  ]

  metadata {
    name      = local.lbc_sa
    namespace = "kube-system"
    labels    = local.lbc_labels

    annotations = {
      "eks.amazonaws.com/role-arn" : aws_iam_role.lbc.arn,
    }
  }
}


resource "helm_release" "aws_lbc" {
  depends_on = [
    null_resource.wait_k8s_api,
    aws_iam_role.lbc,
    aws_iam_role_policy.lbc_policy
  ]

  name       = "aws-lbc"
  repository = "https://aws.github.io/eks-charts"
  chart      = "aws-load-balancer-controller"
  version    = "1.7.2"
  namespace  = "kube-system"

  set {
    name  = "clusterName"
    value = var.name
  }

  set {
    name  = "replicaCount"
    value = "1"
  }

  set {
    name  = "serviceAccount.create"
    value = "false"
  }

  set {
    name  = "serviceAccount.name"
    value = kubernetes_service_account.lbc.metadata[0].name
  }

  set {
    name  = "enableServiceMutatorWebhook"
    value = "false"
  }
}