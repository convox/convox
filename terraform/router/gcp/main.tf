terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 2.12"
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
    CACHE      = "redis"
    REDIS_ADDR = "${google_redis_instance.cache.host}:${google_redis_instance.cache.port}"
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
}
