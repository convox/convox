output "host" {
  depends_on = [kubernetes_stateful_set.elasticsearch]
  value      = "${kubernetes_service.elasticsearch.metadata.0.name}.${var.namespace}.svc.cluster.local"
}

output "url" {
  depends_on = [kubernetes_stateful_set.elasticsearch]
  value      = "http://${kubernetes_service.elasticsearch.metadata.0.name}.${var.namespace}.svc.cluster.local:9200"
}
