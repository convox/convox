locals {
  # Accepts both AWS-style (ON_DEMAND/SPOT) and Azure-style (Regular/Spot) capacity types
  spot_capacity_types = ["SPOT", "Spot"]

  additional_node_groups_with_defaults = [
    for idx, ng in var.additional_node_groups : {
      id           = tonumber(lookup(ng, "id", idx))
      machine_type = lookup(ng, "type", null)
      disk_size    = tonumber(lookup(ng, "disk", var.node_disk))
      disk_type    = lookup(ng, "disk_type", "pd-balanced")
      spot         = contains(local.spot_capacity_types, lookup(ng, "capacity_type", "ON_DEMAND"))
      min_count    = tonumber(lookup(ng, "min_size", 1))
      max_count    = tonumber(lookup(ng, "max_size", 100))
      label        = lookup(ng, "label", null)
      dedicated    = tobool(lookup(ng, "dedicated", false))
      zones        = compact(split(",", lookup(ng, "zones", "")))
      gpu_type     = lookup(ng, "gpu_type", null)
      gpu_count    = tonumber(lookup(ng, "gpu_count", 1))
      resource_labels = {
        for pair in compact(split(",", lookup(ng, "tags", ""))) :
        trimspace(split("=", pair)[0]) => trimspace(try(split("=", pair)[1], "novalue"))
      }
    }
  ]
}

resource "google_container_node_pool" "additional" {
  for_each = { for ng in local.additional_node_groups_with_defaults : ng.id => ng }

  name     = "${var.name}-np${each.key}"
  location = google_container_cluster.rack.location
  cluster  = google_container_cluster.rack.name

  version            = var.k8s_version
  initial_node_count = each.value.min_count
  node_locations     = length(each.value.zones) > 0 ? each.value.zones : null

  autoscaling {
    min_node_count = each.value.min_count
    max_node_count = each.value.max_count
  }

  node_config {
    machine_type    = each.value.machine_type
    spot            = each.value.spot
    disk_size_gb    = each.value.disk_size
    disk_type       = each.value.disk_type
    labels          = each.value.label != null || each.value.dedicated ? { "convox.io/label" = coalesce(each.value.label, "custom") } : {}
    resource_labels = each.value.resource_labels

    metadata = {
      disable-legacy-endpoints = "true"
    }

    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    service_account = google_service_account.nodes.email

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
      "https://www.googleapis.com/auth/devstorage.read_write",
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
    ]

    dynamic "taint" {
      for_each = each.value.dedicated ? [1] : []
      content {
        key    = "dedicated-node"
        value  = coalesce(each.value.label, "custom")
        effect = "NO_SCHEDULE"
      }
    }

    dynamic "guest_accelerator" {
      for_each = each.value.gpu_type != null ? [1] : []
      content {
        type  = each.value.gpu_type
        count = each.value.gpu_count
        gpu_driver_installation_config {
          gpu_driver_version = "DEFAULT"
        }
      }
    }
  }

  upgrade_settings {
    max_surge       = 1
    max_unavailable = 1
  }

  timeouts {
    update = var.terraform_update_timeout
  }

  lifecycle {
    ignore_changes = [initial_node_count]
  }
}
