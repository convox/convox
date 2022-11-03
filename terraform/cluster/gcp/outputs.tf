output "ca" {
  depends_on = [
    google_container_node_pool.rack,
    kubernetes_cluster_role_binding.client,
  ]
  value = base64decode(google_container_cluster.rack.master_auth[0].cluster_ca_certificate)
}

output "client_certificate" {
  depends_on = [
    google_container_node_pool.rack,
    kubernetes_cluster_role_binding.client,
  ]
  value = base64decode(google_container_cluster.rack.master_auth[0].client_certificate)
}

output "client_key" {
  depends_on = [
    google_container_node_pool.rack,
    kubernetes_cluster_role_binding.client,
  ]
  value = base64decode(google_container_cluster.rack.master_auth[0].client_key)
}

output "endpoint" {
  depends_on = [
    google_container_node_pool.rack,
    kubernetes_cluster_role_binding.client,
  ]
  value = "https://${google_container_cluster.rack.endpoint}"
}

output "id" {
  depends_on = [
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
