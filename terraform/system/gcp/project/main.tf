provider "google" {
  version = "~> 2.18"
}

resource "google_project_service" "cloudresourcemanager" {
  disable_on_destroy = false
  service            = "cloudresourcemanager.googleapis.com"
}

resource "google_project_service" "compute" {
  disable_on_destroy = false
  service            = "compute.googleapis.com"
}

resource "google_project_service" "container" {
  disable_on_destroy = false
  service            = "container.googleapis.com"
}

resource "google_project_service" "iam" {
  disable_on_destroy = false
  service            = "iam.googleapis.com"
}

resource "google_project_service" "redis" {
  disable_on_destroy = false
  service            = "redis.googleapis.com"
}
