provider "kubernetes" {
  version = "~> 1.10"
}

provider "random" {
  version = "~> 2.2"
}

resource "random_string" "password" {
  length  = 64
  special = false
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

resource "kubernetes_service_account" "api" {
  metadata {
    namespace = var.namespace
    name      = "api"

    annotations = var.annotations
  }
}

resource "kubernetes_persistent_volume_claim" "api_storage" {
  metadata {
    namespace = var.namespace
    name      = "api-storage"

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
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        storage = "5Gi"
      }
    }
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
        annotations = merge(var.annotations, {
          "scheduler.alpha.kubernetes.io/critical-pod" : ""
        })

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

        container {
          name              = "main"
          args              = ["api"]
          image             = "convox/convox:${var.release}"
          image_pull_policy = "Always"

          env {
            name  = "DOMAIN"
            value = var.domain
          }

          env {
            name  = "IMAGE"
            value = "convox/convox:${var.release}"
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

          volume_mount {
            name       = "docker"
            mount_path = "/var/run/docker.sock"
          }

          volume_mount {
            name       = "storage"
            mount_path = "/var/storage"
          }
        }

        volume {
          name = "docker"

          host_path {
            path = var.socket
          }
        }

        volume {
          name = "storage"

          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.api_storage.metadata.0.name
          }
        }
      }
    }
  }
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

    selector = {
      system  = "convox"
      service = "api"
    }
  }
}

resource "kubernetes_ingress" "api" {
  metadata {
    namespace = var.namespace
    name      = "api"

    annotations = merge({
      "convox.com/backend-protocol" : "https",
      "nginx.ingress.kubernetes.io/backend-protocol" : "https",
    }, var.annotations)

    labels = {
      system  = "convox"
      service = "api"
    }
  }

  spec {
    tls {
      hosts       = ["api.${var.domain}"]
      secret_name = "api-certificate"
    }

    rule {
      host = "api.${var.domain}"

      http {
        path {
          backend {
            service_name = kubernetes_service.api.metadata.0.name
            service_port = 5443
          }
        }
      }
    }
  }
}
