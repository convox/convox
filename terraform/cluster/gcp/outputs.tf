output "ca" {
  depends_on = [google_container_cluster.rack]
  value      = base64decode(google_container_cluster.rack.master_auth.0.cluster_ca_certificate)
}

output "client_certificate" {
  depends_on = [google_container_cluster.rack]
  value      = base64decode(google_container_cluster.rack.master_auth.0.client_certificate)
}

output "client_key" {
  depends_on = [google_container_cluster.rack]
  value      = base64decode(google_container_cluster.rack.master_auth.0.client_key)
}

output "endpoint" {
  depends_on = [google_container_cluster.rack]
  value      = "https://${google_container_cluster.rack.endpoint}"
}

output "nodes_account" {
  depends_on = [
    google_project_service.cloudresourcemanager,
    google_project_service.redis,
  ]

  value = google_service_account.nodes.email
}
