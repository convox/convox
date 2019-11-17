terraform {
  required_version = ">= 0.12.0"
}

provider "kubernetes" {
  version = "~> 1.9"
}

provider "random" {
  version = "~> 2.2"
}

resource "random_string" "password" {
  length  = 64
  special = false
}

resource "null_resource" "crd" {
  provisioner "local-exec" {
    when    = "create"
    command = "kubectl apply -f ${path.module}/crd.yml"
    environment = {
      "KUBECONFIG" : var.kubeconfig,
    }
  }

  provisioner "local-exec" {
    when    = "destroy"
    command = "kubectl delete -f ${path.module}/crd.yml"
    environment = {
      "KUBECONFIG" : var.kubeconfig,
    }
  }

  triggers = {
    template = filesha256("${path.module}/crd.yml")
  }
}

resource "kubernetes_cluster_role" "api" {
  metadata {
    name = "${var.name}-api"
  }

  rule {
    api_groups = ["*"]
    resources  = ["*"]
    verbs      = ["*"]
  }
}

resource "kubernetes_cluster_role_binding" "api" {
  metadata {
    name = "${var.name}-api"
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

resource "kubernetes_deployment" "api" {
  metadata {
    namespace = var.namespace
    name      = "api"

    labels = {
      system  = "convox"
      service = "api"
    }
  }

  spec {
    min_ready_seconds      = 3
    revision_history_limit = 0
    replicas               = 2

    selector {
      match_labels = {
        system  = "convox"
        service = "api"
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
          system  = "convox"
          service = "api"
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
            value = random_string.password.result
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

            failure_threshold     = 3
            initial_delay_seconds = 15
            period_seconds        = 5
            success_threshold     = 1
            timeout_seconds       = 3
          }

          readiness_probe {
            http_get {
              path   = "/check"
              port   = 5443
              scheme = "HTTPS"
            }

            period_seconds  = 5
            timeout_seconds = 3
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
          host_path {
            path = "/var/rack/${var.name}/storage"
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

    annotations = {
      "convox.idles" : "true"
      "convox.ingress.service.api.5443.protocol" : "https"
    }

    labels = {
      system  = "convox"
      service = "api"
    }
  }

  spec {
    tls {
      hosts = ["api.${var.domain}"]
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
