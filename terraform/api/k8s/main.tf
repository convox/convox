resource "random_string" "password" {
  length  = 64
  special = false
}

resource "kubernetes_resource_quota" "gcp-critical-pods" {
  metadata {
    name = "gcp-critical-pods"
    namespace = var.namespace
  }
  spec {
    hard = {
      pods = "1000"
    }
    scope_selector {
      match_expression {
        scope_name = "PriorityClass"
        operator = "In"
        values = ["system-node-critical", "system-cluster-critical"]
      }
    }
  }
}

resource "kubernetes_cluster_role" "api" {
  metadata {
    name = "${var.rack}-api"
  }

  rule {
    api_groups = ["*"]
    resources  = ["*"]
    verbs      = ["*"]
  }
}

resource "kubernetes_cluster_role_binding" "api" {
  metadata {
    name = "${var.rack}-api"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = kubernetes_cluster_role.api.metadata.0.name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account.api.metadata.0.name
    namespace = kubernetes_service_account.api.metadata.0.namespace
  }
}

resource "kubernetes_cluster_role_binding" "api-cluster-admin" {
  metadata {
    name = "${var.rack}-api-cluster-admin"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "cluster-admin"
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account.api.metadata.0.name
    namespace = kubernetes_service_account.api.metadata.0.namespace
  }
}

resource "kubernetes_service_account" "api" {
  metadata {
    namespace = var.namespace
    name      = "api"

    annotations = var.annotations
  }
}

resource "kubernetes_deployment" "api" {
  metadata {
    namespace = var.namespace
    name      = "api"

    labels = {
      app     = "system"
      name    = "api"
      rack    = var.rack
      service = "api"
      system  = "convox"
      type    = "service"
    }
  }

  spec {
    min_ready_seconds      = 3
    revision_history_limit = 0
    replicas               = var.replicas

    selector {
      match_labels = {
        name    = "api"
        service = "api"
        system  = "convox"
        type    = "service"
      }
    }

    strategy {
      type = "RollingUpdate"
      rolling_update {
        max_surge       = 1
        max_unavailable = 0
      }
    }

    template {
      metadata {
        annotations = var.annotations

        labels = merge(var.labels, {
          app     = "system"
          name    = "api"
          rack    = var.rack
          service = "api"
          system  = "convox"
          type    = "service"
        })
      }

      spec {
        automount_service_account_token = true
        service_account_name            = kubernetes_service_account.api.metadata.0.name
        share_process_namespace         = true
        priority_class_name             = "system-cluster-critical"

        dynamic "image_pull_secrets" {
          for_each = var.docker_hub_authentication != "NULL" ? [var.docker_hub_authentication] : []
          content {
            name = var.docker_hub_authentication
          }
        }


        container {
          name              = "system"
          args              = ["api"]
          image             = "${var.image}:${var.release}"
          image_pull_policy = "IfNotPresent"

          env {
            name  = "BUILDKIT_ENABLED"
            value = var.buildkit_enabled
          }

          env {
            name  = "BUILD_NODE_ENABLED"
            value = var.build_node_enabled
          }

          env {
            name  = "CONVOX_DOMAIN_TLS_CERT_DISABLE"
            value = var.convox_domain_tls_cert_disable
          }

          env {
            name  = "DOCKER_HUB_USERNAME"
            value = var.docker_hub_username
          }

          env {
            name  = "DOCKER_HUB_PASSWORD"
            value = var.docker_hub_password
          }

          env {
            name  = "DOMAIN"
            value = var.domain
          }

          env {
            name  = "DOMAIN_INTERNAL"
            value = var.domain_internal
          }

          env {
            name  = "DISABLE_IMAGE_MANIFEST_CACHE"
            value = var.disable_image_manifest_cache
          }


          env {
            name  = "IMAGE"
            value = "${var.image}:${var.release}"
          }

          env {
            name  = "METRICS_SCRAPER_HOST"
            value = var.metrics_scraper_host
          }

          env {
            name = "NAMESPACE"
            value_from {
              field_ref {
                field_path = "metadata.namespace"
              }
            }
          }

          env {
            name  = "PASSWORD"
            value = var.authentication ? random_string.password.result : ""
          }

          env {
            name  = "RACK_NAME"
            value = var.rack_name
          }

          env {
            name  = "VERSION"
            value = var.release
          }

          dynamic "env" {
            for_each = var.env

            content {
              name  = env.key
              value = env.value
            }
          }

          port {
            container_port = 5443
          }

          liveness_probe {
            http_get {
              path   = "/check"
              port   = 5443
              scheme = "HTTPS"
            }

            failure_threshold     = 5
            initial_delay_seconds = 0
            period_seconds        = 3
            success_threshold     = 1
            timeout_seconds       = 3
          }

          readiness_probe {
            http_get {
              path   = "/check"
              port   = 5443
              scheme = "HTTPS"
            }

            failure_threshold     = 5
            initial_delay_seconds = 0
            period_seconds        = 3
            success_threshold     = 1
            timeout_seconds       = 3
          }

          dynamic "volume_mount" {
            for_each = var.volumes
            iterator = volume

            content {
              name       = volume.key
              mount_path = volume.value
            }
          }
        }

        volume {
          name = "docker"

          host_path {
            path = var.socket
          }
        }

        dynamic "volume" {
          for_each = var.volumes

          content {
            name = volume.key

            persistent_volume_claim {
              claim_name = "api-${volume.key}"
            }
          }
        }

        dns_config {
          nameservers = [var.resolver]
        }

        dns_policy = "None"
      }
    }
  }
  depends_on = [ kubernetes_resource_quota.gcp-critical-pods ]
}

resource "kubernetes_service" "api" {
  metadata {
    namespace = var.namespace
    name      = "api"

    labels = {
      system  = "convox"
      service = "api"
    }
  }

  spec {
    port {
      name        = "https"
      port        = 5443
      target_port = 5443
      protocol    = "TCP"
    }

    port {
      name        = "kubernetes"
      port        = 8001
      target_port = 8001
      protocol    = "TCP"
    }

    selector = {
      system  = "convox"
      service = "api"
    }
  }
}

resource "kubernetes_ingress_v1" "api" {
  wait_for_load_balancer = true

  metadata {
    namespace = var.namespace
    name      = "api-ing-v1"

    annotations = merge({
      "convox.com/backend-protocol" : "https",
      "nginx.ingress.kubernetes.io/backend-protocol" : "https",
      "nginx.ingress.kubernetes.io/proxy-read-timeout" : "99999",
      "nginx.ingress.kubernetes.io/proxy-send-timeout" : "99999",
    }, var.annotations)

    labels = {
      system  = "convox"
      service = "api"
    }
  }

  spec {
    ingress_class_name = "nginx"
    tls {
      hosts       = ["api.${var.domain}"]
      secret_name = "api-certificate"
    }

    rule {
      host = "api.${var.domain}"

      http {
        path {
          backend {
            service {
              name = kubernetes_service.api.metadata[0].name
              port {
                number = 5443
              }
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_ingress_v1" "kubernetes" {
  wait_for_load_balancer = true

  metadata {
    namespace = var.namespace
    name      = "kubernetes-ing-v1"

    annotations = merge({
      "nginx.ingress.kubernetes.io/use-regex" : "true",
    }, var.annotations)

    labels = {
      system  = "convox"
      service = "api"
    }
  }

  spec {
    ingress_class_name = "nginx"
    tls {
      hosts       = ["api.${var.domain}"]
      secret_name = "api-certificate"
    }

    rule {
      host = "api.${var.domain}"

      http {
        path {
          path = "/kubernetes/.*"

          backend {
            service {
              name = kubernetes_service.api.metadata[0].name
              port {
                number = 8001
              }
            }
          }
        }
      }
    }
  }
}
