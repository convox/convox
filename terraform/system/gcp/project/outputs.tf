output "id" {
  depends_on = [
    google_project_service.cloudresourcemanager,
    google_project_service.compute,
    google_project_service.container,
    google_project_service.iam,
    google_project_service.redis,
  ]

  value = data.google_client_config.current.project
}

output "region" {
  depends_on = [
    google_project_service.cloudresourcemanager,
    google_project_service.compute,
    google_project_service.container,
    google_project_service.iam,
    google_project_service.redis,
  ]

  value = data.google_client_config.current.region
}
