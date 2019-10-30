resource "google_service_account" "nodes" {
  account_id = "${var.name}-nodes"
}

resource "google_project_iam_member" "nodes-logging" {
  depends_on = ["google_project_service.cloudresourcemanager"]

  role   = "roles/logging.logWriter"
  member = "serviceAccount:${google_service_account.nodes.email}"
}

resource "google_project_iam_member" "nodes-monitoring" {
  depends_on = ["google_project_service.cloudresourcemanager"]

  role   = "roles/monitoring.metricWriter"
  member = "serviceAccount:${google_service_account.nodes.email}"
}

# resource "google_project_iam_member" "nodes-storage" {
#   depends_on = ["google_project_service.cloudresourcemanager"]

#   role   = "roles/storage.admin"
#   member = "serviceAccount:${google_service_account.nodes.email}"
# }

# resource "google_project_iam_member" "nodes-token-creator" {
#   depends_on = ["google_project_service.cloudresourcemanager"]

#   role   = "roles/iam.serviceAccountTokenCreator"
#   member = "serviceAccount:${google_service_account.nodes.email}"
# }
