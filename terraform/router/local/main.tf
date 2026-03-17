data "kubernetes_service_v1" "ingress_nginx" {
  metadata {
    name      = "ingress-nginx-controller"
    namespace = "ingress-nginx"
  }
}

resource "kubernetes_service" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"
  }

  spec {
    type = "ClusterIP"

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
  }
}

resource "kubernetes_endpoints_v1" "router" {
  metadata {
    namespace = var.namespace
    name      = kubernetes_service.router.metadata[0].name
  }

  subset {
    address {
      ip = data.kubernetes_service_v1.ingress_nginx.spec[0].cluster_ip
    }

    port {
      name     = "http"
      port     = 80
      protocol = "TCP"
    }

    port {
      name     = "https"
      port     = 443
      protocol = "TCP"
    }
  }
}

# add nginx global configuration in minikube ingress
# otherwise large file upload fails when running convox build command
resource "kubernetes_config_map_v1_data" "ingress-nginx-controller-configmap" {
  metadata {
    name      = "ingress-nginx-controller"
    namespace = "ingress-nginx"
  }
  data = {
    "hsts"            = "false"
    "proxy-body-size" = "0"
  }
}
