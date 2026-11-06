# NodeOverlay CRs from the karpenter_node_overlays param. The matching feature gate is set in karpenter.tf.
resource "kubectl_manifest" "karpenter_node_overlay" {
  for_each = { for o in var.karpenter_node_overlays : o.name => o if var.karpenter_enabled }

  yaml_body = yamlencode({
    apiVersion = "karpenter.sh/v1alpha1"
    kind       = "NodeOverlay"
    metadata   = { name = each.value.name }
    spec = merge(
      { requirements = each.value.requirements },
      lookup(each.value, "capacity", null) != null ? { capacity = each.value.capacity } : {},
      lookup(each.value, "price", null) != null ? { price = each.value.price } : {},
      lookup(each.value, "priceAdjustment", null) != null ? { priceAdjustment = each.value.priceAdjustment } : {},
      lookup(each.value, "weight", null) != null ? { weight = each.value.weight } : {},
    )
  })

  wait       = true
  depends_on = [null_resource.wait_karpenter_ready, helm_release.karpenter_crd]
}
