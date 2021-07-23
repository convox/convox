resource "kubernetes_cluster_role" "resolver" {
  metadata {
    name = "resolver"
  }

  rule {
    api_groups = ["extensions"]
    resources  = ["ingresses"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = [""]
    resources  = ["services"]
    verbs      = ["get", "list", "watch"]
  }
}

resource "kubernetes_cluster_role_binding" "resolver" {
  depends_on = [kubernetes_cluster_role.resolver, kubernetes_service_account.resolver]

  metadata {
    name = "resolver"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "resolver"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "resolver"
    namespace = var.namespace
  }
}

resource "kubernetes_service_account" "resolver" {
  metadata {
    namespace = var.namespace
    name      = "resolver"

    annotations = var.annotations
  }
}

resource "kubernetes_deployment" "resolver" {
  depends_on = [kubernetes_cluster_role_binding.resolver]

  metadata {
    namespace = var.namespace
    name      = "resolver"

    labels = {
      app     = "system"
      name    = "resolver"
      rack    = var.rack
      service = "resolver"
      system  = "convox"
      type    = "service"
    }
  }

  spec {
    min_ready_seconds      = 1
    revision_history_limit = 1
    replicas               = var.replicas

    selector {
      match_labels = {
        name    = "resolver"
        service = "resolver"
        system  = "convox"
        type    = "service"
      }
    }

    strategy {
      type = "RollingUpdate"
      rolling_update {
        max_surge       = "100%"
        max_unavailable = "0"
      }
    }

    template {
      metadata {
        annotations = var.annotations

        labels = {
          app     = "system"
          name    = "resolver"
          rack    = var.rack
          service = "resolver"
          system  = "convox"
          type    = "service"
        }
      }

      spec {
        automount_service_account_token = true
        service_account_name            = "resolver"
        priority_class_name             = var.set_priority_class ? "system-cluster-critical" : null

        affinity {
          pod_anti_affinity {
            preferred_during_scheduling_ignored_during_execution {
              weight = 100
              pod_affinity_term {
                label_selector {
                  match_labels = {
                    system  = "convox"
                    service = "resolver"
                  }
                }
                topology_key = "kubernetes.io/hostname"
              }
            }
          }
        }

        container {
          name              = "system"
          args              = ["resolver"]
          image             = "${var.image}:${var.release}"
          image_pull_policy = "Always"

          env {
            name = "NAMESPACE"
            value_from {
              field_ref {
                field_path = "metadata.namespace"
              }
            }
          }

          dynamic "env" {
            for_each = var.env

            content {
              name  = env.key
              value = env.value
            }
          }

          port {
            container_port = "5453"
            protocol       = "UDP"
          }

          port {
            container_port = "5454"
            protocol       = "UDP"
          }

          resources {
            requests {
              cpu    = "64m"
              memory = "64Mi"
            }
          }
        }

        dns_config {
          option {
            name  = "ndots"
            value = "1"
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "resolver" {
  metadata {
    namespace = var.namespace
    name      = "resolver"
  }

  spec {
    type = "ClusterIP"

    port {
      name        = "dns"
      port        = 53
      protocol    = "UDP"
      target_port = 5454
    }

    selector = {
      system  = "convox"
      service = "resolver"
    }
  }

  lifecycle {
    ignore_changes = [metadata[0].annotations]
  }
}
