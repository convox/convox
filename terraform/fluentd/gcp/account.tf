resource "google_service_account" "fluentd" {
  account_id = "${var.name}-flientd"
}

resource "google_service_account_key" "fluentd" {
  service_account_id = google_service_account.fluentd.name
}

resource "google_project_iam_member" "fluentd-logging" {
  role   = "roles/logging.admin"
  member = "serviceAccount:${google_service_account.fluentd.email}"
}

resource "google_service_account_iam_binding" "fluentd-binding" {
  service_account_id = google_service_account.fluentd.name
  role               = "roles/iam.serviceAccountTokenCreator"

  members = [
    "serviceAccount:${data.google_client_config.current.project}.svc.id.goog[${var.namespace}/fluentd]"
  ]
}
