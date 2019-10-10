output "endpoint" {
  value = kubernetes_service.router.load_balancer_ingress.0.ip
}

