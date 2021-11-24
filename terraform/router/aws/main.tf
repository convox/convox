locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

data "aws_region" "current" {
}

module "nginx" {
  source = "../nginx"

  providers = {
    kubernetes = kubernetes
  }

  namespace    = var.namespace
  rack         = var.name
  replicas_max = var.high_availability ? 10 : 1
  replicas_min = var.high_availability ? 2 : 1
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"

    annotations = {
      "service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout" = "${var.idle_timeout}"
      # "service.beta.kubernetes.io/aws-load-balancer-proxy-protocol"          = "*"
      "service.beta.kubernetes.io/aws-load-balancer-type" = "nlb"
    }
  }

  spec {
    external_traffic_policy = "Cluster"
    type                    = "LoadBalancer"

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
  url = "https://alias.convox.com/alias/${length(kubernetes_service.router.load_balancer_ingress) > 0 ? kubernetes_service.router.load_balancer_ingress.0.hostname : ""}"
}
