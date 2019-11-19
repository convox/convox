output "kubeconfig" {
  depends_on = [
    local_file.kubeconfig,
    azurerm_kubernetes_cluster.rack,
  ]
  value = local_file.kubeconfig.filename
}

output "workspace" {
  value = azurerm_log_analytics_workspace.rack.workspace_id
}
