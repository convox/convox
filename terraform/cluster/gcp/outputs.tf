output "kubeconfig" {
  depends_on = [
    local_file.kubeconfig,
    kubernetes_cluster_role_binding.client,
    google_container_node_pool.rack,
  ]
  value = local_file.kubeconfig.filename
}

output "nodes_account" {
  depends_on = [
    google_project_service.cloudresourcemanager,
    google_project_service.redis,
  ]

  value = google_service_account.nodes.email
}
