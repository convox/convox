module "nginx" {
  source = "../nginx"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_authentication = var.docker_hub_authentication
  namespace                 = var.namespace
  rack                      = var.name
  replicas_max              = var.high_availability ? 10 : 1
  replicas_min              = var.high_availability ? 2 : 1
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"

    annotations = {
      # the oci cloud controller manager also rewrites the load balancer subnet
      # security list unless it is told to leave it alone
      "service.beta.kubernetes.io/oci-load-balancer-security-list-management-mode" = "None"
      "service.beta.kubernetes.io/oci-load-balancer-shape"                         = "flexible"
      "service.beta.kubernetes.io/oci-load-balancer-shape-flex-min"                = var.lb_bandwidth_min
      "service.beta.kubernetes.io/oci-load-balancer-shape-flex-max"                = var.lb_bandwidth_max
    }
  }

  spec {
    type = "LoadBalancer"

    load_balancer_source_ranges = var.whitelist

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

data "http" "alias" {
  url = "https://alias.convox.com/alias/${length(kubernetes_service.router.status.0.load_balancer.0.ingress) > 0 ? kubernetes_service.router.status.0.load_balancer.0.ingress.0.ip : ""}"
}
