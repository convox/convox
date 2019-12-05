output "resolver" {
  depends_on = [kubernetes_service.resolver]
  value      = kubernetes_service.resolver.spec.0.cluster_ip
}
