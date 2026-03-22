# Karpenter NodePool and EC2NodeClass resources
# All resources gated on var.karpenter_enabled

locals {
  karpenter_effective_disk = var.karpenter_node_disk > 0 ? var.karpenter_node_disk : var.node_disk

  # Parse workload NodePool custom labels from "k1=v1,k2=v2" to map
  karpenter_workload_extra_labels = {
    for pair in compact(split(",", var.karpenter_node_labels)) :
    trimspace(split("=", pair)[0]) => trimspace(split("=", pair)[1])
  }

  # Parse workload NodePool custom taints from "key=value:Effect" to list of objects
  karpenter_workload_taints = [
    for t in compact(split(",", var.karpenter_node_taints)) : {
      key    = split("=", split(":", t)[0])[0]
      value  = length(split("=", split(":", t)[0])) > 1 ? split("=", split(":", t)[0])[1] : ""
      effect = element(split(":", t), length(split(":", t)) - 1)
    }
  ]

  # Parse build NodePool extra labels
  karpenter_build_extra_labels = {
    for pair in compact(split(",", var.karpenter_build_node_labels)) :
    trimspace(split("=", pair)[0]) => trimspace(split("=", pair)[1])
  }

  ###########################################################################
  # karpenter_config overrides — decode customer JSON (empty = no overrides)
  ###########################################################################

  karpenter_config_raw      = var.karpenter_config != "" ? var.karpenter_config : "{}"
  karpenter_config_parsed   = jsondecode(local.karpenter_config_raw)
  kc_np                     = lookup(local.karpenter_config_parsed, "nodePool", {})
  kc_np_template            = lookup(local.kc_np, "template", {})
  kc_np_template_meta       = lookup(local.kc_np_template, "metadata", {})
  kc_np_template_spec       = lookup(local.kc_np_template, "spec", {})
  kc_np_disruption          = lookup(local.kc_np, "disruption", {})
  kc_ec2                    = lookup(local.karpenter_config_parsed, "ec2NodeClass", {})

  ###########################################################################
  # Workload NodePool — build defaults, merge overrides, force protected fields
  ###########################################################################

  # Requirements: use override if provided, otherwise build from individual params
  np_default_requirements = concat(
    [
      { key = "karpenter.sh/capacity-type", operator = "In", values = split(",", var.karpenter_capacity_types) },
      { key = "kubernetes.io/arch", operator = "In", values = split(",", var.karpenter_arch) },
    ],
    var.karpenter_instance_families != "" ? [
      { key = "karpenter.k8s.aws/instance-family", operator = "In", values = split(",", var.karpenter_instance_families) }
    ] : [],
    var.karpenter_instance_sizes != "" ? [
      { key = "karpenter.k8s.aws/instance-size", operator = "In", values = split(",", var.karpenter_instance_sizes) }
    ] : [],
  )
  np_final_requirements = lookup(local.kc_np_template_spec, "requirements", local.np_default_requirements)

  # Labels: always include convox.io/nodepool, merge param labels, then override labels
  np_final_labels = merge(
    { "convox.io/nodepool" = "workload" },
    local.karpenter_workload_extra_labels,
    lookup(local.kc_np_template_meta, "labels", {})
  )

  # Taints: use override if provided, otherwise param taints
  np_final_taints = lookup(local.kc_np_template_spec, "taints", (
    length(local.karpenter_workload_taints) > 0 ? local.karpenter_workload_taints : null
  ))

  # expireAfter: override or param
  np_final_expire = lookup(local.kc_np_template_spec, "expireAfter", var.karpenter_node_expiry)

  # terminationGracePeriod: only from override (no individual param)
  np_final_term_grace = lookup(local.kc_np_template_spec, "terminationGracePeriod", null)

  # Limits: override or params
  np_final_limits = lookup(local.kc_np, "limits", {
    cpu    = tostring(var.karpenter_cpu_limit)
    memory = "${var.karpenter_memory_limit_gb}Gi"
  })

  # Disruption: override or params
  np_default_disruption = {
    consolidationPolicy = var.karpenter_consolidation_enabled ? "WhenEmptyOrUnderutilized" : "WhenEmpty"
    consolidateAfter    = var.karpenter_consolidate_after
    budgets             = [{ nodes = var.karpenter_disruption_budget_nodes }]
  }
  np_final_disruption = length(local.kc_np_disruption) > 0 ? local.kc_np_disruption : local.np_default_disruption

  # Weight: override or default (0 = not set)
  np_final_weight = lookup(local.kc_np, "weight", null)

  # Build the template.spec — only include non-null fields
  np_template_spec_map = merge(
    {
      expireAfter  = local.np_final_expire
      nodeClassRef = { group = "karpenter.k8s.aws", kind = "EC2NodeClass", name = "workload" }
      requirements = local.np_final_requirements
    },
    local.np_final_taints != null ? { taints = local.np_final_taints } : {},
    local.np_final_term_grace != null ? { terminationGracePeriod = local.np_final_term_grace } : {},
  )

  # Full NodePool manifest
  np_workload_manifest = {
    apiVersion = "karpenter.sh/v1"
    kind       = "NodePool"
    metadata   = { name = "workload" }
    spec = merge(
      {
        template = {
          metadata = { labels = local.np_final_labels }
          spec     = local.np_template_spec_map
        }
        limits     = local.np_final_limits
        disruption = local.np_final_disruption
      },
      local.np_final_weight != null ? { weight = local.np_final_weight } : {},
    )
  }

  ###########################################################################
  # Workload EC2NodeClass — build defaults, merge overrides, force protected fields
  ###########################################################################

  # blockDeviceMappings: override or params
  ec2_default_block_devices = [{
    deviceName = "/dev/xvda"
    ebs = {
      volumeType = var.karpenter_node_volume_type
      volumeSize = "${local.karpenter_effective_disk}Gi"
      encrypted  = var.ebs_volume_encryption_enabled
    }
  }]
  ec2_final_block_devices = lookup(local.kc_ec2, "blockDeviceMappings", local.ec2_default_block_devices)

  # metadataOptions: override or params
  ec2_default_metadata = {
    httpTokens                 = var.imds_http_tokens
    httpPutResponseHopLimit    = var.imds_http_hop_limit
    httpEndpoint               = "enabled"
  }
  ec2_final_metadata = lookup(local.kc_ec2, "metadataOptions", local.ec2_default_metadata)

  # Tags: always include Name + Rack + rack tags, merge customer overrides
  ec2_final_tags = merge(
    { Name = "${var.name}/karpenter/workload", Rack = var.name },
    local.tags,
    lookup(local.kc_ec2, "tags", {})
  )

  # amiSelectorTerms: override or default
  ec2_final_ami = lookup(local.kc_ec2, "amiSelectorTerms", [{ alias = "al2023@latest" }])

  # Optional fields from override only (no individual params for these)
  ec2_optional_fields = merge(
    lookup(local.kc_ec2, "userData", null) != null ? { userData = local.kc_ec2["userData"] } : {},
    lookup(local.kc_ec2, "detailedMonitoring", null) != null ? { detailedMonitoring = local.kc_ec2["detailedMonitoring"] } : {},
    lookup(local.kc_ec2, "associatePublicIPAddress", null) != null ? { associatePublicIPAddress = local.kc_ec2["associatePublicIPAddress"] } : {},
    lookup(local.kc_ec2, "instanceStorePolicy", null) != null ? { instanceStorePolicy = local.kc_ec2["instanceStorePolicy"] } : {},
  )

  # Full EC2NodeClass manifest — protected fields forced after merge
  ec2_workload_manifest = {
    apiVersion = "karpenter.k8s.aws/v1"
    kind       = "EC2NodeClass"
    metadata   = { name = "workload" }
    spec = merge(
      {
        # Protected: Convox always controls these
        role = var.karpenter_enabled ? aws_iam_role.karpenter_nodes[0].name : ""
        subnetSelectorTerms         = [{ tags = { "karpenter.sh/discovery" = var.name } }]
        securityGroupSelectorTerms  = [{ tags = { "karpenter.sh/discovery" = var.name } }]
        # Configurable with defaults
        amiSelectorTerms     = local.ec2_final_ami
        blockDeviceMappings  = local.ec2_final_block_devices
        metadataOptions      = local.ec2_final_metadata
        tags                 = local.ec2_final_tags
      },
      local.ec2_optional_fields,
    )
  }

  ###########################################################################
  # Process additional Karpenter NodePools — apply defaults and parse labels/taints
  ###########################################################################

  additional_karpenter_nodepools_with_defaults = {
    for idx, np in var.additional_karpenter_nodepools : lookup(np, "name", "custom-${idx}") => {
      name                  = lookup(np, "name", "custom-${idx}")
      instance_families     = lookup(np, "instance_families", "")
      instance_sizes        = lookup(np, "instance_sizes", "")
      capacity_types        = lookup(np, "capacity_types", "on-demand")
      arch                  = lookup(np, "arch", "amd64")
      cpu_limit             = tonumber(lookup(np, "cpu_limit", 100))
      memory_limit_gb       = tonumber(lookup(np, "memory_limit_gb", 400))
      consolidation_policy  = lookup(np, "consolidation_policy", "WhenEmptyOrUnderutilized")
      consolidate_after     = lookup(np, "consolidate_after", "30s")
      node_expiry           = lookup(np, "node_expiry", "720h")
      disruption_budget_nodes = lookup(np, "disruption_budget_nodes", "10%")
      disk                  = tonumber(lookup(np, "disk", 0))
      volume_type           = lookup(np, "volume_type", "gp3")
      weight                = tonumber(lookup(np, "weight", 0))
      labels = {
        for pair in compact(split(",", lookup(np, "labels", ""))) :
        trimspace(split("=", pair)[0]) => trimspace(split("=", pair)[1])
      }
      taints = [
        for t in compact(split(",", lookup(np, "taints", ""))) : {
          key    = split("=", split(":", t)[0])[0]
          value  = length(split("=", split(":", t)[0])) > 1 ? split("=", split(":", t)[0])[1] : ""
          effect = element(split(":", t), length(split(":", t)) - 1)
        }
      ]
    }
  }
}

###############################################################################
# Workload NodePool — built from defaults + karpenter_config overrides
###############################################################################

resource "kubectl_manifest" "karpenter_nodepool_workload" {
  count     = var.karpenter_enabled ? 1 : 0
  yaml_body = yamlencode(local.np_workload_manifest)
  depends_on = [helm_release.karpenter]
}

###############################################################################
# Workload EC2NodeClass — built from defaults + karpenter_config overrides
###############################################################################

resource "kubectl_manifest" "karpenter_ec2nodeclass_workload" {
  count     = var.karpenter_enabled ? 1 : 0
  yaml_body = yamlencode(local.ec2_workload_manifest)
  depends_on = [helm_release.karpenter]
}

###############################################################################
# Build NodePool (conditional on build_node_enabled AND karpenter_enabled)
###############################################################################

resource "kubectl_manifest" "karpenter_nodepool_build" {
  count = var.build_node_enabled && var.karpenter_enabled ? 1 : 0

  yaml_body = templatefile("${path.module}/templates/karpenter-nodepool-build.yaml.tpl", {
    karpenter_node_expiry             = var.karpenter_node_expiry
    karpenter_build_capacity_types    = var.karpenter_build_capacity_types
    karpenter_build_instance_families = var.karpenter_build_instance_families
    karpenter_build_instance_sizes    = var.karpenter_build_instance_sizes
    karpenter_build_cpu_limit         = var.karpenter_build_cpu_limit
    karpenter_build_consolidate_after = var.karpenter_build_consolidate_after
    extra_labels                      = local.karpenter_build_extra_labels
  })

  depends_on = [helm_release.karpenter]
}

resource "kubectl_manifest" "karpenter_ec2nodeclass_build" {
  count = var.build_node_enabled && var.karpenter_enabled ? 1 : 0

  yaml_body = templatefile("${path.module}/templates/karpenter-ec2nodeclass.yaml.tpl", {
    name                       = "build"
    cluster_name               = var.name
    karpenter_node_role_name   = aws_iam_role.karpenter_nodes[0].name
    karpenter_node_volume_type = var.karpenter_node_volume_type
    karpenter_effective_disk   = local.karpenter_effective_disk
    ebs_encrypted              = var.ebs_volume_encryption_enabled
    imds_http_tokens           = var.imds_http_tokens
    imds_http_hop_limit        = var.imds_http_hop_limit
    extra_tags                 = local.tags
  })

  depends_on = [helm_release.karpenter]
}

###############################################################################
# Additional Custom NodePools (via additional_karpenter_nodepools_config)
###############################################################################

resource "kubectl_manifest" "karpenter_nodepool_additional" {
  for_each = var.karpenter_enabled ? local.additional_karpenter_nodepools_with_defaults : {}

  yaml_body = templatefile("${path.module}/templates/karpenter-nodepool-custom.yaml.tpl", {
    name                  = each.value.name
    instance_families     = each.value.instance_families
    instance_sizes        = each.value.instance_sizes
    capacity_types        = each.value.capacity_types
    arch                  = each.value.arch
    cpu_limit             = each.value.cpu_limit
    memory_limit_gb       = each.value.memory_limit_gb
    consolidation_policy  = each.value.consolidation_policy
    consolidate_after     = each.value.consolidate_after
    node_expiry           = each.value.node_expiry
    disruption_budget_nodes = each.value.disruption_budget_nodes
    weight                = each.value.weight
    labels                = each.value.labels
    taints                = each.value.taints
  })

  depends_on = [helm_release.karpenter]
}

resource "kubectl_manifest" "karpenter_ec2nodeclass_additional" {
  for_each = var.karpenter_enabled ? local.additional_karpenter_nodepools_with_defaults : {}

  yaml_body = templatefile("${path.module}/templates/karpenter-ec2nodeclass.yaml.tpl", {
    name                       = each.value.name
    cluster_name               = var.name
    karpenter_node_role_name   = aws_iam_role.karpenter_nodes[0].name
    karpenter_node_volume_type = each.value.volume_type
    karpenter_effective_disk   = each.value.disk > 0 ? each.value.disk : local.karpenter_effective_disk
    ebs_encrypted              = var.ebs_volume_encryption_enabled
    imds_http_tokens           = var.imds_http_tokens
    imds_http_hop_limit        = var.imds_http_hop_limit
    extra_tags                 = local.tags
  })

  depends_on = [helm_release.karpenter]
}
