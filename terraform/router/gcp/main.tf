locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

module "nginx" {
  source = "../nginx"

  providers = {
    kubernetes = kubernetes
  }

  nginx_additional_config   = var.nginx_additional_config
  docker_hub_authentication = var.docker_hub_authentication
  namespace                 = var.namespace
  rack                      = var.name
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"
  }

  spec {
    type = "LoadBalancer"

    load_balancer_source_ranges = var.whitelist

    port {
      name        = "http"
      port        = 80
      protocol    = "TCP"
      target_port = 80
    }

    port {
      name        = "https"
      port        = 443
      protocol    = "TCP"
      target_port = 443
    }

    selector = module.nginx.selector
  }
}

data "http" "alias" {
  url = "https://alias.convox.com/alias/${length(kubernetes_service.router.status.0.load_balancer.0.ingress) > 0 ? kubernetes_service.router.status.0.load_balancer.0.ingress.0.ip : ""}"
}
