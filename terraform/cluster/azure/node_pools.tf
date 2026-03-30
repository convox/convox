locals {
  # Map capacity_type values to Azure priority values
  # Accepts both AWS-style (ON_DEMAND/SPOT) and Azure-native (Regular/Spot)
  capacity_type_map = {
    "ON_DEMAND" = "Regular"
    "SPOT"      = "Spot"
    "Regular"   = "Regular"
    "Spot"      = "Spot"
  }

  additional_node_groups_with_defaults = [
    for idx, ng in var.additional_node_groups : {
      id           = tonumber(lookup(ng, "id", idx))
      vm_size      = lookup(ng, "type", null)
      os_disk_size = tonumber(lookup(ng, "disk", var.node_disk))
      priority     = lookup(local.capacity_type_map, lookup(ng, "capacity_type", "ON_DEMAND"), "Regular")
      min_count    = tonumber(lookup(ng, "min_size", 1))
      max_count    = tonumber(lookup(ng, "max_size", 100))
      label        = lookup(ng, "label", null)
      dedicated    = tobool(lookup(ng, "dedicated", false))
      zones        = compact(split(",", lookup(ng, "zones", "")))
      tags = {
        for pair in compact(split(",", lookup(ng, "tags", ""))) :
        trimspace(split("=", pair)[0]) => trimspace(try(split("=", pair)[1], "novalue"))
      }
    }
  ]

  additional_build_groups_with_defaults = [
    for idx, ng in var.additional_build_groups : {
      id           = tonumber(lookup(ng, "id", idx))
      vm_size      = lookup(ng, "type", null)
      os_disk_size = tonumber(lookup(ng, "disk", var.node_disk))
      priority     = lookup(local.capacity_type_map, lookup(ng, "capacity_type", "ON_DEMAND"), "Regular")
      min_count    = tonumber(lookup(ng, "min_size", 0))
      max_count    = tonumber(lookup(ng, "max_size", 100))
      label        = lookup(ng, "label", null)
      zones        = compact(split(",", lookup(ng, "zones", "")))
      tags = {
        for pair in compact(split(",", lookup(ng, "tags", ""))) :
        trimspace(split("=", pair)[0]) => trimspace(try(split("=", pair)[1], "novalue"))
      }
    }
  ]
}

###### additional node pools

resource "azurerm_kubernetes_cluster_node_pool" "additional" {
  for_each = { for ng in local.additional_node_groups_with_defaults : ng.id => ng }

  kubernetes_cluster_id       = azurerm_kubernetes_cluster.rack.id
  name                        = "np${each.key}"
  vm_size                     = each.value.vm_size
  os_disk_size_gb             = each.value.os_disk_size
  auto_scaling_enabled        = true
  min_count                   = each.value.min_count
  max_count                   = each.value.max_count
  priority                    = each.value.priority
  eviction_policy             = each.value.priority == "Spot" ? "Delete" : null
  spot_max_price              = each.value.priority == "Spot" ? -1 : null
  temporary_name_for_rotation = "nptemp${each.key}"
  zones                       = length(each.value.zones) > 0 ? each.value.zones : null

  node_labels = merge(
    each.value.label != null ? { "convox.io/label" = each.value.label } : {},
    each.value.priority == "Spot" ? { "kubernetes.azure.com/scalesetpriority" = "spot" } : {}
  )

  dynamic "node_network_profile" {
    for_each = []
    content {}
  }

  node_taints = concat(
    each.value.dedicated ? ["dedicated-node=${coalesce(each.value.label, "custom")}:NoSchedule"] : [],
    each.value.priority == "Spot" ? ["kubernetes.azure.com/scalesetpriority=spot:NoSchedule"] : []
  )

  tags = length(each.value.tags) > 0 ? each.value.tags : null

  timeouts {
    update = var.terraform_update_timeout
  }

  lifecycle {
    ignore_changes = [node_count]
  }
}

###### additional build node pools

resource "azurerm_kubernetes_cluster_node_pool" "build_additional" {
  for_each = { for ng in local.additional_build_groups_with_defaults : ng.id => ng }

  kubernetes_cluster_id       = azurerm_kubernetes_cluster.rack.id
  name                        = "bp${each.key}"
  vm_size                     = each.value.vm_size
  os_disk_size_gb             = each.value.os_disk_size
  auto_scaling_enabled        = true
  min_count                   = each.value.min_count
  max_count                   = each.value.max_count
  priority                    = each.value.priority
  eviction_policy             = each.value.priority == "Spot" ? "Delete" : null
  spot_max_price              = each.value.priority == "Spot" ? -1 : null
  temporary_name_for_rotation = "bptemp${each.key}"
  zones                       = length(each.value.zones) > 0 ? each.value.zones : null

  node_labels = merge(
    { "convox-build" = "true" },
    each.value.label != null ? { "convox.io/label" = each.value.label } : { "convox.io/label" = "custom-build" }
  )

  node_taints = [
    "dedicated=build:NoSchedule",
  ]

  tags = length(each.value.tags) > 0 ? each.value.tags : null

  timeouts {
    update = var.terraform_update_timeout
  }

  lifecycle {
    ignore_changes = [node_count]
  }
}
