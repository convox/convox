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
