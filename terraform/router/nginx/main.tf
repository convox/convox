provider "kubernetes" {
  version = "~> 1.10"
}

resource "kubernetes_config_map" "nginx-configuration" {
  metadata {
    namespace = var.namespace
    name      = "nginx-configuration"
  }

  data = {
    "proxy-body-size" = "0"
    # "use-proxy-protocol" = "true"
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

resource "kubernetes_cluster_role" "ingress-nginx" {
  metadata {
    name = "ingress-nginx"
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps", "endpoints", "nodes", "pods", "secrets"]
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
    api_groups = ["extensions", "networking.k8s.io"]
    resources  = ["ingresses/status"]
    verbs      = ["update"]
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
    resources  = ["configmaps", "pods", "secrets", "namespaces"]
    verbs      = ["get"]
  }

  rule {
    api_groups     = [""]
    resources      = ["configmaps"]
    resource_names = ["ingress-controller-leader-nginx"]
    verbs          = ["get", "update"]
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps"]
    verbs      = ["create"]
  }

  rule {
    api_groups = [""]
    resources  = ["endpoints"]
    verbs      = ["get"]
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
          app     = "system"
          name    = "ingress-nginx"
          rack    = var.rack
          system  = "convox"
          service = "ingress-nginx"
          type    = "service"
        }
      }

      spec {
        termination_grace_period_seconds = 300
        service_account_name             = "ingress-nginx"
        automount_service_account_token  = true

        container {
          name  = "main"
          image = "quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.26.1"
          args = [
            "/nginx-ingress-controller",
            "--configmap=$(POD_NAMESPACE)/nginx-configuration",
            "--tcp-services-configmap=$(POD_NAMESPACE)/tcp-services",
            "--udp-services-configmap=$(POD_NAMESPACE)/udp-services",
            "--publish-service=$(POD_NAMESPACE)/router",
            "--annotations-prefix=nginx.ingress.kubernetes.io",
          ]

          security_context {
            allow_privilege_escalation = true
            capabilities {
              drop = ["ALL"]
              add  = ["NET_BIND_SERVICE"]
            }
            run_as_user = 33
          }

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

          port {
            name           = "http"
            container_port = 80
          }

          port {
            name           = "https"
            container_port = 443
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
