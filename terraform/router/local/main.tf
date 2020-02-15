terraform {
  required_version = ">= 0.12.0"
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.10"
}

provider "tls" {
  version = "~> 2.1"
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
  rack      = var.name
  release   = var.release

  env = {
    CACHE = "memory"
  }
}

resource "kubernetes_service" "resolver-external" {
  metadata {
    namespace = var.namespace
    name      = "resolver-external"
  }

  spec {
    type = var.platform == "Linux" ? "ClusterIP" : "LoadBalancer"

    port {
      name        = "dns"
      port        = 53
      protocol    = "UDP"
      target_port = 5453
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

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"
  }

  spec {
    type = var.platform == "Linux" ? "ClusterIP" : "LoadBalancer"

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
