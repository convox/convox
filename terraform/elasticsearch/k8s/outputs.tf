output "host" {
  value = "${kubernetes_service.elasticsearch.metadata.0.name}.${var.namespace}.svc.cluster.local"
}

output "url" {
  value = "http://${kubernetes_service.elasticsearch.metadata.0.name}.${var.namespace}.svc.cluster.local:9200"
}
