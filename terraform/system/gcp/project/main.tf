provider "google" {
  alias = "direct"
}

data "google_client_config" "current" {
  provider = google.direct
}

resource "google_project_service" "cloudresourcemanager" {
  provider = google.direct

  disable_on_destroy = false
  service            = "cloudresourcemanager.googleapis.com"
}

resource "google_project_service" "compute" {
  provider = google.direct

  disable_on_destroy = false
  service            = "compute.googleapis.com"
}

resource "google_project_service" "container" {
  provider = google.direct

  disable_on_destroy = false
  service            = "container.googleapis.com"
}

resource "google_project_service" "iam" {
  provider = google.direct

  disable_on_destroy = false
  service            = "iam.googleapis.com"
}

resource "google_project_service" "redis" {
  provider = google.direct

  disable_on_destroy = false
  service            = "redis.googleapis.com"
}
