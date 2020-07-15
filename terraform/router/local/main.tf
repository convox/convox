provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.11"
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

module "nginx" {
  source = "../nginx"

  providers = {
    kubernetes = kubernetes
  }

  namespace    = var.namespace
  rack         = var.name
  replicas_min = 1
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

    selector = module.nginx.selector
  }
}
