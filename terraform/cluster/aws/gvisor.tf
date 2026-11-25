###############################################################################
# gVisor for sandboxes (opt-in per node pool with "gvisor": true)
#
# A node group in additional_node_groups_config or a pool in additional_karpenter_nodepools_config
# with "gvisor": true boots with the pinned gVisor release installed and registered with containerd,
# and gets the convox.io/sandbox-runtime=gvisor label. The gvisor RuntimeClass schedules only onto
# those nodes. Pools without the flag render exactly the user data they did before.
###############################################################################

locals {
  # https://gvisor.dev/docs/user_guide/install/ — release tarballs and their published SHA-512s.
  gvisor_release        = "20261116"
  gvisor_sha512_x86_64  = "c87d792d5842efb70c38ee822124cfe0821a547715e7abcf3f2b14997cd02784edaa0b91e238ec5c27a25a1b9e00bf98113398eb62a0778d8d20d4e222f8b0e0"
  gvisor_sha512_aarch64 = "a64ce2d5ba6825a0a6459ee6a9ce40164749c2d276a945b885673988c4e545e66b2f4ef3a08e068a26cca676061c8c5e8ba982156feb786f0eab995f5b8b854d"

  gvisor_node_label_key   = "convox.io/sandbox-runtime"
  gvisor_node_label_value = "gvisor"

  gvisor_node_groups     = { for ng in local.additional_node_groups_with_defaults : ng.id => ng if ng.gvisor }
  gvisor_karpenter_pools = var.karpenter_enabled ? { for k, np in local.additional_karpenter_nodepools_with_defaults : k => np if np.gvisor } : {}
  gvisor_enabled         = length(local.gvisor_node_groups) > 0 || length(local.gvisor_karpenter_pools) > 0

  gvisor_template_vars = {
    gvisor_release            = local.gvisor_release
    gvisor_sha512_x86_64      = local.gvisor_sha512_x86_64
    gvisor_sha512_aarch64     = local.gvisor_sha512_aarch64
    fast_image_pull           = var.fast_image_pull_enable
    kubelet_registry_set      = local.kubelet_registry_set
    kubelet_registry_pull_qps = local.kubelet_registry_pull_qps_effective
    kubelet_registry_burst    = local.kubelet_registry_burst_effective
  }

  # Managed node groups also carry the rack's user_data / user_data_url scripts, as their other
  # launch templates do. Karpenter classes never have, so theirs carry only gVisor and NodeConfig.
  gvisor_node_group_user_data = templatefile("${path.module}/files/gvisor_user_data.sh", merge(local.gvisor_template_vars, {
    user_data             = var.user_data
    user_data_script_file = var.user_data_url != "" ? data.http.user_data_content[0].response_body : ""
  }))
  gvisor_karpenter_user_data = templatefile("${path.module}/files/gvisor_user_data.sh", merge(local.gvisor_template_vars, {
    user_data             = ""
    user_data_script_file = ""
  }))
}

resource "kubectl_manifest" "gvisor_runtime_class" {
  count = local.gvisor_enabled ? 1 : 0

  yaml_body = yamlencode({
    apiVersion = "node.k8s.io/v1"
    kind       = "RuntimeClass"
    metadata   = { name = "gvisor" }
    handler    = "runsc"
    scheduling = {
      nodeSelector = { (local.gvisor_node_label_key) = local.gvisor_node_label_value }
      tolerations = [{
        key      = "dedicated-node"
        operator = "Exists"
        effect   = "NoSchedule"
      }]
    }
  })

  depends_on = [null_resource.wait_k8s_api]
}
