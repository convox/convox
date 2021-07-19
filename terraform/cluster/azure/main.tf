data "azurerm_kubernetes_service_versions" "available" {
  location       = var.region
  version_prefix = "1.17."
}

resource "random_string" "suffix" {
  length  = 6
  special = false
  upper   = false
}

resource "azurerm_log_analytics_workspace" "rack" {
  name                = "${var.name}-${random_string.suffix.result}"
  location            = var.region
  resource_group_name = var.name
  sku                 = "PerGB2018"
  retention_in_days   = 30

  tags = {
    resource_group_id = var.resource_group
  }
}

resource "azurerm_kubernetes_cluster" "rack" {
  depends_on = [azurerm_role_assignment.cluster-contributor]

  name                = var.name
  location            = var.region
  resource_group_name = var.name
  dns_prefix          = var.name
  kubernetes_version  = data.azurerm_kubernetes_service_versions.available.latest_version

  default_node_pool {
    enable_auto_scaling = true
    name                = "default"
    min_count           = 3
    max_count           = 100
    node_count          = 3
    vm_size             = var.node_type
    os_disk_size_gb     = 30
  }

  addon_profile {
    oms_agent {
      enabled                    = true
      log_analytics_workspace_id = azurerm_log_analytics_workspace.rack.id
    }
  }

  service_principal {
    client_id     = azuread_service_principal.cluster.application_id
    client_secret = azuread_service_principal_password.cluster.value
  }

  lifecycle {
    ignore_changes = [default_node_pool[0].node_count]
  }
}
