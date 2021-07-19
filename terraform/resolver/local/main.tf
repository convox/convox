module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  image     = var.image
  namespace = var.namespace
  rack      = var.rack
  release   = var.release
  replicas  = 1
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

    selector = module.k8s.selector
  }

  lifecycle {
    ignore_changes = [metadata[0].annotations]
  }
}
