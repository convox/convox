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


# One document per file from the chart's crds/crds.yaml, tracking the chart
# version below. Helm installs CRDs only on a first install. Never deleted:
# dropping the TargetGroupBinding CRD cascades to every binding in the cluster.
resource "kubectl_manifest" "lbc_crds" {
  for_each = fileset("${path.module}/files/lbc-crds", "*.yaml")

  depends_on = [null_resource.wait_eks_addons]

  yaml_body         = file("${path.module}/files/lbc-crds/${each.value}")
  server_side_apply = true
  force_conflicts   = true
  apply_only        = true
}


resource "helm_release" "aws_lbc" {
  depends_on = [
    null_resource.wait_eks_addons,
    aws_iam_role.lbc,
    aws_iam_role_policy.lbc_policy,
    aws_eks_node_group.cluster,
    kubectl_manifest.lbc_crds,
  ]

  name       = "aws-lbc"
  repository = "https://aws.github.io/eks-charts"
  chart      = "aws-load-balancer-controller"
  version    = "3.5.0"
  namespace  = "kube-system"
  timeout    = 600

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

  # Chart 3.x defaults this to false, which mints a new webhook CA on every
  # render while the running pod still serves the old one.
  set {
    name  = "keepTLSSecret"
    value = "true"
  }

  # These default to on, and the controller exits if Gateway setup fails.
  # Setting them explicitly also pins them: the controller's startup probe
  # only adjusts gates that were left at their default.
  set {
    name  = "controllerConfig.featureGates.NLBGatewayAPI"
    value = "false"
  }

  set {
    name  = "controllerConfig.featureGates.ALBGatewayAPI"
    value = "false"
  }

  set {
    name  = "controllerConfig.featureGates.GatewayListenerSet"
    value = "false"
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "nodeSelector.convox\\.io/system-node"
      value = "true"
      type  = "string"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "tolerations[0].key"
      value = "convox.io/system-node"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "tolerations[0].operator"
      value = "Equal"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "tolerations[0].value"
      value = "true"
      type  = "string"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "tolerations[0].effect"
      value = "NoSchedule"
    }
  }
}
