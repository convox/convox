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

  cloud_provider = "aws"
  namespace      = var.namespace
  proxy_protocol = var.proxy_protocol
  rack           = var.name
  replicas_max   = var.high_availability ? 10 : 1
  replicas_min   = var.high_availability ? 2 : 1
}

resource "kubernetes_config_map" "nginx-configuration" {
  metadata {
    namespace = var.namespace
    name      = "nginx-configuration"
  }

  data = {
    "proxy-body-size"    = "0"
    "use-proxy-protocol" = var.proxy_protocol ? "true" : "false"
  }

  depends_on = [
    null_resource.set_proxy_protocol
  ]
}

resource "null_resource" "set_proxy_protocol" {

  triggers = {
    proxy_protocol = var.proxy_protocol
  }

  provisioner "local-exec" {
    command = "sh ${path.module}/proxy-protocol.sh ${var.name} ${var.proxy_protocol} ${data.aws_region.current.name}"
  }

  depends_on = [
    kubernetes_service.router
  ]
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"

    annotations = {
      "service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout" = "${var.idle_timeout}"
      "service.beta.kubernetes.io/aws-load-balancer-type"                    = "nlb"
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
  url = "https://alias.convox.com/alias/${length(kubernetes_service.router.status.0.load_balancer.0.ingress) > 0 ? kubernetes_service.router.status.0.load_balancer.0.ingress.0.hostname : ""}"
}
