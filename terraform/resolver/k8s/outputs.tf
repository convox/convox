output "endpoint" {
  depends_on = [kubernetes_service.resolver]
  value      = kubernetes_service.resolver.spec.0.cluster_ip
}

output "selector" {
  value = {
    service = "resolver"
    system  = "convox"
  }
}
