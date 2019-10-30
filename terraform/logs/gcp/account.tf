resource "google_service_account" "logs" {
  account_id = "${var.name}-logs"
}

resource "google_service_account_key" "logs" {
  service_account_id = google_service_account.logs.name
}

resource "google_project_iam_member" "logs-logging" {
  role   = "roles/logging.admin"
  member = "serviceAccount:${google_service_account.logs.email}"
}

resource "google_service_account_iam_binding" "logs-binding" {
  service_account_id = google_service_account.logs.name
  role               = "roles/iam.serviceAccountTokenCreator"

  members = [
    "serviceAccount:${data.google_client_config.current.project}.svc.id.goog[${var.namespace}/fluentd]"
  ]
}
