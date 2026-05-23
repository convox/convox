resource "helm_release" "contour" {
  depends_on = [
    null_resource.wait_k8s_api,
    aws_eks_node_group.cluster,
  ]

  count = var.router_type == "contour" ? 1 : 0

  name             = "contour"
  namespace        = "projectcontour"
  repository       = "https://projectcontour.github.io/helm-charts/"
  chart            = "contour"
  version          = "0.5.0"
  create_namespace = true
  timeout          = 600

  set {
    name  = "contour.ingressClass.name"
    value = "contour"
  }

  set {
    name  = "contour.ingressClass.default"
    value = "false"
  }

  set {
    name  = "envoy.service.type"
    value = "ClusterIP"
  }

  set {
    name  = "envoy.service.externalTrafficPolicy"
    value = ""
  }

  set {
    name  = "contour.resources.requests.cpu"
    value = "100m"
  }

  set {
    name  = "contour.resources.requests.memory"
    value = "128Mi"
  }

  set {
    name  = "envoy.resources.requests.cpu"
    value = "100m"
  }

  set {
    name  = "envoy.resources.requests.memory"
    value = "128Mi"
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "contour.nodeSelector.convox\\.io/system-node"
      value = "true"
      type  = "string"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "contour.tolerations[0].key"
      value = "convox.io/system-node"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "contour.tolerations[0].operator"
      value = "Equal"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "contour.tolerations[0].value"
      value = "true"
      type  = "string"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "contour.tolerations[0].effect"
      value = "NoSchedule"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "envoy.nodeSelector.convox\\.io/system-node"
      value = "true"
      type  = "string"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "envoy.tolerations[0].key"
      value = "convox.io/system-node"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "envoy.tolerations[0].operator"
      value = "Equal"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "envoy.tolerations[0].value"
      value = "true"
      type  = "string"
    }
  }

  dynamic "set" {
    for_each = var.karpenter_enabled ? [1] : []
    content {
      name  = "envoy.tolerations[0].effect"
      value = "NoSchedule"
    }
  }
}
