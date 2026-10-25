locals {
  vpa_system_node_selector = var.karpenter_enabled ? { "convox.io/system-node" = "true" } : {}
  vpa_system_tolerations = var.karpenter_enabled ? [{
    key      = "convox.io/system-node"
    operator = "Equal"
    value    = "true"
    effect   = "NoSchedule"
  }] : []
}

# Helm installs crds/ only on a first install; these track the chart version below.
resource "kubectl_manifest" "vpa_crds" {
  for_each = var.vpa_enable ? fileset("${path.module}/files/vpa-crds", "*.yaml") : toset([])

  depends_on = [null_resource.wait_eks_addons]

  yaml_body         = file("${path.module}/files/vpa-crds/${each.value}")
  server_side_apply = true
  force_conflicts   = true
  apply_only        = true
}

resource "helm_release" "vpa" {
  depends_on = [
    null_resource.wait_eks_addons,
    aws_eks_node_group.cluster,
    kubectl_manifest.vpa_crds,
  ]

  count            = var.vpa_enable ? 1 : 0
  name             = "vpa"
  repository       = "https://charts.fairwinds.com/stable"
  chart            = "vpa"
  version          = "5.1.0"
  namespace        = "vpa"
  create_namespace = true
  timeout          = 600

  values = [
    yamlencode({
      admissionController = {
        image = {
          tag = "1.7.3@sha256:404b5e7389ae85881c8e06147035244c9f0aa366d5b1b10cbb1ca8eda72ee07e"
        }
        nodeSelector = local.vpa_system_node_selector
        tolerations  = local.vpa_system_tolerations
        certGen = {
          nodeSelector = local.vpa_system_node_selector
          tolerations  = local.vpa_system_tolerations
        }
      }

      updater = {
        image = {
          tag = "1.7.3@sha256:666e5fe1b933c85c0bc01dfca9cd7fac93bcc5618072e32d6bece16b77e39d14"
        }
        nodeSelector = local.vpa_system_node_selector
        tolerations  = local.vpa_system_tolerations
      }

      recommender = {
        image = {
          tag = "1.7.3@sha256:3b207e6e719e8a88b96b5777afa6f6fb261e73669fa39f487c04560d85bf2e7d"
        }
        nodeSelector = local.vpa_system_node_selector
        tolerations  = local.vpa_system_tolerations
      }
    })
  ]
}
