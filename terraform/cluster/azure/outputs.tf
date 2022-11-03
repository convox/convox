output "ca" {
  value = base64decode(azurerm_kubernetes_cluster.rack.kube_config.0.cluster_ca_certificate)
}

output "client_certificate" {
  value = base64decode(azurerm_kubernetes_cluster.rack.kube_config.0.client_certificate)
}

output "client_key" {
  value = base64decode(azurerm_kubernetes_cluster.rack.kube_config.0.client_key)
}

output "endpoint" {
  value = azurerm_kubernetes_cluster.rack.kube_config.0.host
}

output "id" {
  value = azurerm_kubernetes_cluster.rack.name
}

output "workspace" {
  value = azurerm_log_analytics_workspace.rack.workspace_id
}
