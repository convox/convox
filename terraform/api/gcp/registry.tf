resource "google_artifact_registry_repository" "docker_repo" {
  location      = var.region
  repository_id = var.name
  description   = "Docker repository for rack ${var.name}"
  format        = "DOCKER"
}

output "repository_url" {
  value = "${var.region}-docker.pkg.dev/${var.project_id}/${var.name}"
}
