output "url" {
  depends_on = [kubernetes_stateful_set.elasticsearch]
  value      = "http://${kubernetes_service.http.metadata.0.name}.${var.namespace}.svc.cluster.local:9200"
}
