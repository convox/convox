provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.10"
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

  namespace = var.namespace
  rack      = var.name
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
      target_port = "http"
    }

    port {
      name        = "https"
      port        = 443
      protocol    = "TCP"
      target_port = "https"
    }

    selector = module.nginx.selector
  }

  lifecycle {
    ignore_changes = [metadata[0].annotations]
  }
}
