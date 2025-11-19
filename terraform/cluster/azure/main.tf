data "azurerm_kubernetes_service_versions" "available" {
  location       = var.region
  version_prefix = "${var.k8s_version}."
}

resource "random_string" "suffix" {
  length  = 6
  special = false
  upper   = false
}

resource "azurerm_log_analytics_workspace" "rack" {
  name                = "${var.name}-${random_string.suffix.result}"
  location            = var.resource_group_location
  resource_group_name = var.resource_group_name
  sku                 = "PerGB2018"
  retention_in_days   = 30

  tags = {
    resource_group_id = var.resource_group
  }
}

resource "azurerm_kubernetes_cluster" "rack" {
  depends_on = [azurerm_role_assignment.cluster-contributor]

  name                = var.name
  location            = var.resource_group_location
  resource_group_name = var.resource_group_name
  dns_prefix          = var.name
  kubernetes_version  = data.azurerm_kubernetes_service_versions.available.latest_version

  default_node_pool {
    auto_scaling_enabled        = true
    name                        = "default"
    temporary_name_for_rotation = "defaulttemp"
    min_count                   = 3
    max_count                   = 100
    node_count                  = 3
    vm_size                     = var.node_type
    os_disk_size_gb             = 30
    orchestrator_version        = data.azurerm_kubernetes_service_versions.available.latest_version
  }

  service_principal {
    client_id     = azuread_service_principal.cluster.client_id
    client_secret = azuread_service_principal_password.cluster.value
  }

  lifecycle {
    ignore_changes = [default_node_pool[0].node_count]
  }
}

# resource "local_file" "kubeconfig" {
#   depends_on = [
#     azurerm_kubernetes_cluster.rack,
#   ]

#   filename = pathexpand("~/.kube/config.azure.${var.name}")
#   content = templatefile("${path.module}/kubeconfig.tpl", {
#     ca                 = azurerm_kubernetes_cluster.rack.kube_config[0].cluster_ca_certificate
#     endpoint           = azurerm_kubernetes_cluster.rack.kube_config[0].host
#     client_certificate = azurerm_kubernetes_cluster.rack.kube_config[0].client_certificate
#     client_key         = azurerm_kubernetes_cluster.rack.kube_config[0].client_key
#   })

#   lifecycle {
#     ignore_changes = [content]
#   }
# }
