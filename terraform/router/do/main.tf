terraform {
  required_version = ">= 0.12.0"
}

provider "digitalocean" {
  version = "~> 1.9"
}

provider "kubernetes" {
  version = "~> 1.9"
}

locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  namespace = var.namespace
  release   = var.release

  env = {
    CACHE        = "redis"
    REDIS_ADDR   = "${digitalocean_database_cluster.cache.private_host}:${digitalocean_database_cluster.cache.port}"
    REDIS_AUTH   = digitalocean_database_cluster.cache.password
    REDIS_SECURE = "true"
  }
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"
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

    selector = {
      system  = "convox"
      service = "router"
    }
  }

  lifecycle {
    ignore_changes = [metadata[0].annotations]
  }
}

data "http" "alias" {
  url = "https://alias.convox.com/alias/${kubernetes_service.router.load_balancer_ingress.0.ip}"
}
