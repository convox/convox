resource "null_resource" "contour_cleanup" {
  count = var.router_type == "contour" ? 1 : 0

  provisioner "local-exec" {
    command = <<-EOT
      kubectl api-resources --api-group=projectcontour.io -o name 2>/dev/null | grep -q httpproxies && \
        kubectl delete httpproxies.projectcontour.io -l system=convox -A --ignore-not-found || true
    EOT
  }
}

resource "helm_release" "contour" {
  count = var.router_type == "contour" ? 1 : 0

  depends_on = [null_resource.contour_cleanup]

  name             = "contour"
  namespace        = var.namespace
  repository       = "https://projectcontour.github.io/helm-charts/"
  chart            = "contour"
  version          = "0.5.0"
  create_namespace = false
  timeout          = 600
  wait             = true

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

  set {
    name  = "configInline.timeouts.connection-idle-timeout"
    value = "${var.idle_timeout}s"
  }

  set {
    name  = "configInline.timeouts.stream-idle-timeout"
    value = "300s"
  }

  set {
    name  = "configInline.network.num-trusted-hops"
    value = "1"
  }

  set {
    name  = "configInline.tls.minimum-protocol-version"
    value = "1.2"
  }

  dynamic "set" {
    for_each = var.proxy_protocol ? [1] : []
    content {
      name  = "contour.extraArgs[0]"
      value = "--use-proxy-protocol"
    }
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
