output "ca" {
  depends_on = [azurerm_kubernetes_cluster.rack]
  value      = base64decode(azurerm_kubernetes_cluster.rack.kube_config.0.cluster_ca_certificate)
}

output "client_certificate" {
  depends_on = [azurerm_kubernetes_cluster.rack]
  value      = base64decode(azurerm_kubernetes_cluster.rack.kube_config.0.client_certificate)
}

output "client_key" {
  depends_on = [azurerm_kubernetes_cluster.rack]
  value      = base64decode(azurerm_kubernetes_cluster.rack.kube_config.0.client_key)
}

output "endpoint" {
  depends_on = [azurerm_kubernetes_cluster.rack]
  value      = azurerm_kubernetes_cluster.rack.kube_config.0.host
}

output "workspace" {
  value = azurerm_log_analytics_workspace.rack.workspace_id
}
