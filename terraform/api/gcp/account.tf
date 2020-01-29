resource "google_service_account" "api" {
  account_id = "${var.name}-api"
}

resource "google_service_account_key" "api" {
  service_account_id = google_service_account.api.name
}

resource "google_project_iam_member" "api-logging-admin" {
  role   = "roles/logging.admin"
  member = "serviceAccount:${google_service_account.api.email}"
}

resource "google_project_iam_member" "api-storage" {
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.api.email}"
}

resource "google_service_account_iam_binding" "api-binding" {
  service_account_id = google_service_account.api.name
  role               = "roles/iam.serviceAccountTokenCreator"

  members = [
    "serviceAccount:${data.google_client_config.current.project}.svc.id.goog[${var.namespace}/api]"
  ]
}
