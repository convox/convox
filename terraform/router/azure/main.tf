locals {
  tags = merge(var.tags, {
    System = "convox"
    Rack   = var.name
  })

  default_nginx_image = "registry.k8s.io/ingress-nginx/controller:v1.12.0@sha256:e6b8de175acda6ca913891f0f727bca4527e797d52688cbe9fec9040d6f6b6fa"
  nginx_image         = var.nginx_image != "" ? var.nginx_image : local.default_nginx_image
}

module "nginx" {
  source = "../nginx"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_authentication = var.docker_hub_authentication
  internal_router           = var.internal_router
  namespace                 = var.namespace
  nginx_image               = local.nginx_image
  nginx_additional_config   = var.nginx_additional_config
  proxy_protocol            = var.proxy_protocol
  rack                      = var.name
  replicas_max              = var.high_availability ? 10 : 1
  replicas_min              = var.high_availability ? 2 : 1
  ssl_ciphers               = var.ssl_ciphers
  ssl_protocols             = var.ssl_protocols
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"

    annotations = {
      "service.beta.kubernetes.io/azure-load-balancer-idle-timeout" = tostring(var.idle_timeout)
    }
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

  lifecycle {
    ignore_changes = [metadata[0].annotations]
  }
}

data "http" "alias" {
  url = "https://alias.convox.com/alias/${length(kubernetes_service.router.status.0.load_balancer.0.ingress) > 0 ? kubernetes_service.router.status.0.load_balancer.0.ingress.0.ip : ""}"
}

resource "kubernetes_service" "router-internal" {
  count = var.internal_router ? 1 : 0

  metadata {
    namespace = var.namespace
    name      = "router-internal"

    annotations = {
      "service.beta.kubernetes.io/azure-load-balancer-internal"     = "true"
      "service.beta.kubernetes.io/azure-load-balancer-idle-timeout" = tostring(var.idle_timeout)
    }
  }

  spec {
    type = "LoadBalancer"

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

    selector = module.nginx.selector-internal
  }

  lifecycle {
    ignore_changes = [metadata[0].annotations]
  }
}

data "http" "alias-internal" {
  count = var.internal_router ? 1 : 0
  url   = "https://alias.convox.com/alias/${length(kubernetes_service.router-internal[0].status.0.load_balancer.0.ingress) > 0 ? kubernetes_service.router-internal[0].status.0.load_balancer.0.ingress.0.ip : ""}"
}
