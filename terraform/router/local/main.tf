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

  nginx_image        = "quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.26.1"
  nginx_user         = "33"
  namespace          = var.namespace
  rack               = var.name
  replicas_min       = 1
  set_priority_class = false
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
