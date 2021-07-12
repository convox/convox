output "endpoint" {
  depends_on = [
    google_container_cluster.rack,
    google_container_node_pool.rack,
    kubernetes_cluster_role_binding.client,
  ]
  value = "https://${google_container_cluster.rack.endpoint}"
}

output "id" {
  depends_on = [
    google_container_cluster.rack,
    google_container_node_pool.rack,
    kubernetes_cluster_role_binding.client,
  ]
  value = google_container_cluster.rack.name
}

output "network" {
  depends_on = [
    google_container_cluster.rack,
    google_container_node_pool.rack,
    kubernetes_cluster_role_binding.client,
  ]
  value = google_compute_network.rack.name
}

output "nodes_account" {
  value = google_service_account.nodes.email
}

output "token" {
  value = data.google_client_config.current.access_token
} 

output "cluster_ca_certificate" {
  value = google_container_cluster.rack.master_auth[0].cluster_ca_certificate
}
