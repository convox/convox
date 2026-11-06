# Prerequisite: The ingress-nginx addon must be enabled and the
# ingress-nginx-controller service must exist in the ingress-nginx
# namespace before this module is applied.
data "kubernetes_service_v1" "ingress_nginx" {
  metadata {
    name      = "ingress-nginx-controller"
    namespace = "ingress-nginx"
  }
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_authentication = var.docker_hub_authentication
  env = {
    ROUTER_IP_OVERRIDE = data.kubernetes_service_v1.ingress_nginx.spec[0].cluster_ip
  }
  image              = var.image
  namespace          = var.namespace
  rack               = var.rack
  release            = var.release
  replicas           = 1
  set_priority_class = false
}

resource "kubernetes_service" "resolver-external" {
  metadata {
    namespace = var.namespace
    name      = "resolver-external"
  }

  spec {
    type = "ClusterIP"

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
