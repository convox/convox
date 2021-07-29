output "endpoint" {
  value = "${length(kubernetes_service.router.status) > 0 ? kubernetes_service.router.status.0.load_balancer.0.ingress.0.ip : ""}.nip.io"
}
