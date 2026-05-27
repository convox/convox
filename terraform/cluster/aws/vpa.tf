locals {
  vpa_system_node_selector = var.karpenter_enabled ? { "convox.io/system-node" = "true" } : {}
  vpa_system_tolerations = var.karpenter_enabled ? [{
    key      = "convox.io/system-node"
    operator = "Equal"
    value    = "true"
    effect   = "NoSchedule"
  }] : []
}

resource "helm_release" "vpa" {
  depends_on = [
    null_resource.wait_eks_addons,
    aws_eks_node_group.cluster,
  ]

  count            = var.vpa_enable ? 1 : 0
  name             = "vpa"
  repository       = "https://charts.fairwinds.com/stable"
  chart            = "vpa"
  version          = "4.10.1"
  namespace        = "vpa"
  create_namespace = true
  timeout          = 600

  values = [
    yamlencode({
      admissionController = {
        extraArgs = {
          feature-gates = "InPlaceOrRecreate=true"
        }
        nodeSelector = local.vpa_system_node_selector
        tolerations  = local.vpa_system_tolerations
        certGen = {
          nodeSelector = local.vpa_system_node_selector
          tolerations  = local.vpa_system_tolerations
        }
      }

      updater = {
        extraArgs = {
          feature-gates = "InPlaceOrRecreate=true"
        }
        nodeSelector = local.vpa_system_node_selector
        tolerations  = local.vpa_system_tolerations
      }

      recommender = {
        nodeSelector = local.vpa_system_node_selector
        tolerations  = local.vpa_system_tolerations
      }
    })
  ]
}
