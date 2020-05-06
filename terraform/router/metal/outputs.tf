output "endpoint" {
  value = "${length(kubernetes_service.router.load_balancer_ingress) > 0 ? kubernetes_service.router.load_balancer_ingress.0.ip : ""}.xip.io"
}
