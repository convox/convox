terraform {
  required_version = ">= 0.12.0"
}

provider "aws" {
  version = "~> 2.49"
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.11"
}

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

  namespace = var.namespace
  rack      = var.name
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"

    annotations = {
      "service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout" = "3600"
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
