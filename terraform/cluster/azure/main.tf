terraform {
  required_version = ">= 0.12.0"
}

provider "azurerm" {
  version = "~> 1.37"
}

provider "local" {
  version = "~> 1.3"
}

provider "random" {
  version = "~> 2.2"
}

data "azurerm_resource_group" "system" {
  name = var.resource_group
}

data "azurerm_kubernetes_service_versions" "available" {
  location       = var.region
  version_prefix = "1.14."
}

resource "random_string" "suffix" {
  length  = 6
  special = false
  upper   = false
}

resource "azurerm_log_analytics_workspace" "rack" {
  name                = "${var.name}-${random_string.suffix.result}"
  location            = data.azurerm_resource_group.system.location
  resource_group_name = data.azurerm_resource_group.system.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
}

resource "azurerm_kubernetes_cluster" "rack" {
  depends_on = [azurerm_role_assignment.cluster-contributor]

  name                = var.name
  location            = data.azurerm_resource_group.system.location
  resource_group_name = data.azurerm_resource_group.system.name
  dns_prefix          = var.name
  kubernetes_version  = data.azurerm_kubernetes_service_versions.available.latest_version

  default_node_pool {
    enable_auto_scaling = true
    name                = "default"
    min_count           = 1
    max_count           = 100
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
}

resource "local_file" "kubeconfig" {
  depends_on = [
    azurerm_kubernetes_cluster.rack,
  ]

  filename = pathexpand("~/.kube/config.azure.${var.name}")
  content = templatefile("${path.module}/kubeconfig.tpl", {
    ca                 = azurerm_kubernetes_cluster.rack.kube_config.0.cluster_ca_certificate
    endpoint           = azurerm_kubernetes_cluster.rack.kube_config.0.host
    client_certificate = azurerm_kubernetes_cluster.rack.kube_config.0.client_certificate
    client_key         = azurerm_kubernetes_cluster.rack.kube_config.0.client_key
  })

  lifecycle {
    ignore_changes = [content]
  }
}
