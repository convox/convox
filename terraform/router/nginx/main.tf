resource "kubernetes_config_map" "nginx-configuration" {
  count = var.cloud_provider == "aws" ? 0 : 1
  metadata {
    namespace = var.namespace
    name      = "nginx-configuration"
  }

  data = {
    "proxy-body-size"    = "0"
    "use-proxy-protocol" = var.proxy_protocol ? "true" : "false"
    "ssl-ciphers"        = var.ssl_ciphers == "" ? null : var.ssl_ciphers
    "ssl-protocols"      = var.ssl_protocols == "" ? null : var.ssl_protocols
  }
}


resource "kubernetes_config_map" "tcp-services" {
  metadata {
    namespace = var.namespace
    name      = "tcp-services"
  }
}


resource "kubernetes_config_map" "udp-services" {
  metadata {
    namespace = var.namespace
    name      = "udp-services"
  }
}

resource "kubernetes_service_account" "ingress-nginx" {
  metadata {
    namespace = var.namespace
    name      = "ingress-nginx"
  }
}

resource "kubernetes_ingress_class" "nginx" {
  metadata {
    name = "nginx"
  }

  spec {
    controller = "k8s.io/ingress-nginx"
  }
}

resource "kubernetes_cluster_role" "ingress-nginx" {
  metadata {
    name = "ingress-nginx"
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps", "endpoints", "nodes", "pods", "secrets", "namespaces"]
    verbs      = ["list", "watch"]
  }

  rule {
    api_groups = ["coordination.k8s.io"]
    resources  = ["leases"]
    verbs      = ["list", "watch"]
  }

  rule {
    api_groups = [""]
    resources  = ["nodes"]
    verbs      = ["get"]
  }

  rule {
    api_groups = [""]
    resources  = ["services"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = [""]
    resources  = ["events"]
    verbs      = ["create", "patch"]
  }

  rule {
    api_groups = ["extensions", "networking.k8s.io"]
    resources  = ["ingresses"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = ["networking.k8s.io"]
    resources  = ["ingressclasses"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = ["extensions", "networking.k8s.io"]
    resources  = ["ingresses/status"]
    verbs      = ["update"]
  }

  rule {
    api_groups = ["discovery.k8s.io"]
    resources  = ["endpointslices"]
    verbs      = ["list", "watch", "get"]
  }

}

resource "kubernetes_cluster_role_binding" "ingress-nginx" {
  metadata {
    name = "ingress-nginx"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "ingress-nginx"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "ingress-nginx"
    namespace = var.namespace
  }
}

resource "kubernetes_role" "ingress-nginx" {
  metadata {
    namespace = var.namespace
    name      = "ingress-nginx"
  }

  rule {
    api_groups = [""]
    resources  = ["namespaces"]
    verbs      = ["get"]
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps", "pods", "secrets", "endpoints", "services"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = ["networking.k8s.io"]
    resources  = ["ingresses", "ingressclasses"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = ["networking.k8s.io"]
    resources  = ["ingresses/status"]
    verbs      = ["update"]
  }

  rule {
    api_groups     = [""]
    resources      = ["configmaps"]
    resource_names = ["ingress-controller-leader-nginx", "ingress-internal-controller-leader-nginx"]
    verbs          = ["get", "update"]
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps"]
    verbs      = ["create"]
  }

  rule {
    api_groups = [""]
    resources  = ["events"]
    verbs      = ["create", "patch"]
  }

  rule {
    api_groups     = ["coordination.k8s.io"]
    resource_names = ["ingress-controller-leader", "ingress-internal-controller-leader", "ingress-nginx-leader", "ingress-nginx-internal-leader"]
    resources      = ["leases"]
    verbs          = ["get", "update"]
  }

  rule {
    api_groups = ["coordination.k8s.io"]
    resources  = ["leases"]
    verbs      = ["create"]
  }

  rule {
    api_groups     = [""]
    resource_names = ["ingress-controller-leader", "ingress-internal-controller-leader"]
    resources      = ["configmaps"]
    verbs          = ["get", "update"]
  }

  rule {
    api_groups     = ["discovery.k8s.io"]
    resources      = ["endpointslices"]
    verbs          = ["get", "list", "watch"]
  }
}

resource "kubernetes_role_binding" "ingress-nginx" {
  metadata {
    namespace = var.namespace
    name      = "ingress-nginx"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = "ingress-nginx"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "ingress-nginx"
    namespace = var.namespace
  }
}

resource "kubernetes_deployment" "ingress-nginx" {
  metadata {
    namespace = var.namespace
    name      = "ingress-nginx"
    labels = {
      app     = "system"
      name    = "ingress-nginx"
      rack    = var.rack
      system  = "convox"
      service = "ingress-nginx"
      type    = "service"
    }
  }

  spec {
    selector {
      match_labels = {
        system  = "convox"
        service = "ingress-nginx"
      }
    }

    template {
      metadata {
        labels = {
          app            = "system"
          name           = "ingress-nginx"
          rack           = var.rack
          system         = "convox"
          service        = "ingress-nginx"
          type           = "service"
          proxy_protocol = var.proxy_protocol
        }
      }

      spec {
        termination_grace_period_seconds = 300
        service_account_name             = "ingress-nginx"
        automount_service_account_token  = true
        priority_class_name              = var.set_priority_class ? "system-cluster-critical" : null

        dynamic "image_pull_secrets" {
          for_each = var.docker_hub_authentication != "NULL" ? [var.docker_hub_authentication] : []
          content {
            name = var.docker_hub_authentication
          }
        }

        container {
          name  = "system"
          image = var.nginx_image
          args = [
            "/nginx-ingress-controller",
            "--watch-ingress-without-class=true",
            "--election-id=ingress-nginx-leader",
            "--configmap=$(POD_NAMESPACE)/nginx-configuration",
            "--tcp-services-configmap=$(POD_NAMESPACE)/tcp-services",
            "--udp-services-configmap=$(POD_NAMESPACE)/udp-services",
            "--publish-service=$(POD_NAMESPACE)/router",
            "--annotations-prefix=nginx.ingress.kubernetes.io",
            "--controller-class=k8s.io/ingress-nginx",
            "--ingress-class=nginx",
          ]

          env {
            name = "POD_NAME"
            value_from {
              field_ref {
                field_path = "metadata.name"
              }
            }
          }

          env {
            name = "POD_NAMESPACE"
            value_from {
              field_ref {
                field_path = "metadata.namespace"
              }
            }
          }

          env {
            name = "LD_PRELOAD"
            value = "/usr/local/lib/libmimalloc.so"
          }

          port {
            name           = "http"
            container_port = 80
          }

          port {
            name           = "https"
            container_port = 443
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "90Mi"
            }
          }

          liveness_probe {
            initial_delay_seconds = 10
            period_seconds        = 10
            timeout_seconds       = 10
            success_threshold     = 1
            failure_threshold     = 3

            http_get {
              path   = "/healthz"
              port   = 10254
              scheme = "HTTP"
            }
          }

          security_context {
            allow_privilege_escalation = false
            capabilities {
              add = [ "NET_BIND_SERVICE" ]
              drop = [ "ALL" ]
            }
            read_only_root_filesystem = false
            run_as_group = 82
            run_as_non_root = true
            run_as_user = 101
            seccomp_profile {
              type = "RuntimeDefault"
            }
          }

          readiness_probe {
            period_seconds    = 10
            timeout_seconds   = 10
            success_threshold = 1
            failure_threshold = 3

            http_get {
              path   = "/healthz"
              port   = 10254
              scheme = "HTTP"
            }
          }

          lifecycle {
            pre_stop {
              exec {
                command = ["/wait-shutdown"]
              }
            }
          }
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [spec[0].replicas]
  }
}


resource "kubernetes_horizontal_pod_autoscaler" "router" {
  metadata {
    namespace = var.namespace
    name      = "nginx"
  }

  spec {
    min_replicas                      = var.replicas_min
    max_replicas                      = var.replicas_max
    target_cpu_utilization_percentage = 90

    scale_target_ref {
      api_version = "apps/v1"
      kind        = "Deployment"
      name        = "ingress-nginx"
    }
  }
}

resource "kubernetes_ingress_class" "nginx-internal" {
  metadata {
    name = "nginx-internal"
  }

  spec {
    controller = "k8s.io/ingress-nginx-internal"
  }
}

resource "kubernetes_deployment" "ingress-nginx-internal" {
  count = var.internal_router ? 1 : 0

  metadata {
    namespace = var.namespace
    name      = "ingress-nginx-internal"
    annotations = {
      "managed-by" = "convox"
    }
  }

  spec {
    replicas = 1
    selector {
      match_labels = {
        system  = "convox"
        service = "ingress-nginx-internal"
      }
    }

    template {
      metadata {
        annotations = {
          "managed-by" = "convox"
        }

        labels = {
          app            = "system"
          name           = "ingress-nginx-internal"
          rack           = var.rack
          system         = "convox"
          service        = "ingress-nginx-internal"
          type           = "service"
          proxy_protocol = var.proxy_protocol
        }
      }

      spec {
        termination_grace_period_seconds = 300
        service_account_name             = "ingress-nginx"
        automount_service_account_token  = true
        priority_class_name              = var.set_priority_class ? "system-cluster-critical" : null

        container {
          name  = "system"
          image = var.nginx_image
          args = [
            "/nginx-ingress-controller",
            "--watch-ingress-without-class=false",
            "--election-id=ingress-nginx-internal-leader",
            "--configmap=$(POD_NAMESPACE)/nginx-internal-configuration",
            "--tcp-services-configmap=$(POD_NAMESPACE)/tcp-services",
            "--udp-services-configmap=$(POD_NAMESPACE)/udp-services",
            "--publish-service=$(POD_NAMESPACE)/router-internal",
            "--annotations-prefix=nginx.ingress.kubernetes.io",
            "--controller-class=k8s.io/ingress-nginx-internal",
            "--ingress-class=nginx-internal",
          ]

          env {
            name = "POD_NAME"
            value_from {
              field_ref {
                field_path = "metadata.name"
              }
            }
          }

          env {
            name = "POD_NAMESPACE"
            value_from {
              field_ref {
                field_path = "metadata.namespace"
              }
            }
          }

          env {
            name = "LD_PRELOAD"
            value = "/usr/local/lib/libmimalloc.so"
          }

          port {
            name           = "http"
            container_port = 80
          }

          port {
            name           = "https"
            container_port = 443
          }

          resources {
            requests = {
              cpu    = "100m"
              memory = "90Mi"
            }
          }

          liveness_probe {
            initial_delay_seconds = 10
            period_seconds        = 10
            timeout_seconds       = 10
            success_threshold     = 1
            failure_threshold     = 3

            http_get {
              path   = "/healthz"
              port   = 10254
              scheme = "HTTP"
            }
          }

          readiness_probe {
            period_seconds    = 10
            timeout_seconds   = 10
            success_threshold = 1
            failure_threshold = 3

            http_get {
              path   = "/healthz"
              port   = 10254
              scheme = "HTTP"
            }
          }

          security_context {
            allow_privilege_escalation = false
            capabilities {
              add = [ "NET_BIND_SERVICE" ]
              drop = [ "ALL" ]
            }
            read_only_root_filesystem = false
            run_as_group = 82
            run_as_non_root = true
            run_as_user = 101
            seccomp_profile {
              type = "RuntimeDefault"
            }
          }

          lifecycle {
            pre_stop {
              exec {
                command = ["/wait-shutdown"]
              }
            }
          }
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [spec[0].replicas]
  }
}

resource "kubernetes_horizontal_pod_autoscaler" "router-internal" {
  count = var.internal_router ? 1 : 0
  metadata {
    namespace = var.namespace
    name      = "nginx-internal"
  }

  spec {
    min_replicas                      = var.replicas_min
    max_replicas                      = var.replicas_max
    target_cpu_utilization_percentage = 90

    scale_target_ref {
      api_version = "apps/v1"
      kind        = "Deployment"
      name        = "ingress-nginx-internal"
    }
  }
}
