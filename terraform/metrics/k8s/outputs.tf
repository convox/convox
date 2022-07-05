output "metrics_scraper_host" {
  value = "http://${kubernetes_service.metrics_scraper.metadata.0.name}.${kubernetes_service.metrics_scraper.metadata.0.namespace}.svc.cluster.local:8000"
}
