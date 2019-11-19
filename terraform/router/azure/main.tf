terraform {
  required_version = ">= 0.12.0"
}

provider "azurerm" {
  version = "~> 1.36"
}

provider "http" {
  version = "~> 1.1"
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

data "azurerm_resource_group" "rack" {
  name = var.resource_group
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
    REDIS_ADDR   = "${azurerm_redis_cache.cache.hostname}:${azurerm_redis_cache.cache.ssl_port}"
    REDIS_AUTH   = azurerm_redis_cache.cache.primary_access_key
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
