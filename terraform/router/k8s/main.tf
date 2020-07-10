provider "kubernetes" {
  version = "~> 1.11"
}

resource "kubernetes_cluster_role" "router" {
  metadata {
    name = "router"
  }

  rule {
    api_groups = [""]
    resources  = ["services"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = ["extensions"]
    resources  = ["deployments", "ingresses"]
    verbs      = ["get", "list", "watch", ]
  }

  rule {
    api_groups = ["extensions"]
    resources  = ["deployments/scale", "ingresses/status"]
    verbs      = ["update"]
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps"]
    verbs      = ["create", "get", "update"]
  }

  rule {
    api_groups = [""]
    resources  = ["events"]
    verbs      = ["create"]
  }
}

resource "kubernetes_cluster_role_binding" "router" {
  depends_on = [kubernetes_cluster_role.router, kubernetes_service_account.router]

  metadata {
    name = "router"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "router"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "router"
    namespace = var.namespace
  }
}

resource "kubernetes_service_account" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"

    annotations = var.annotations
  }
}

resource "kubernetes_deployment" "router" {
  depends_on = [kubernetes_cluster_role_binding.router]

  metadata {
    namespace = var.namespace
    name      = "router"

    labels = {
      app     = "system"
      name    = "router"
      rack    = var.rack
      service = "router"
      system  = "convox"
      type    = "service"
    }
  }

  spec {
    min_ready_seconds      = 1
    revision_history_limit = 1

    selector {
      match_labels = {
        name    = "router"
        service = "router"
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
          name    = "router"
          rack    = var.rack
          service = "router"
          system  = "convox"
          type    = "service"
        }
      }

      spec {
        automount_service_account_token = true
        service_account_name            = "router"

        affinity {
          pod_anti_affinity {
            preferred_during_scheduling_ignored_during_execution {
              weight = 100
              pod_affinity_term {
                label_selector {
                  match_labels = {
                    system  = "convox"
                    service = "router"
                  }
                }
                topology_key = "kubernetes.io/hostname"
              }
            }
          }
        }

        container {
          name              = "main"
          args              = ["router"]
          image             = "convox/convox:${var.release}"
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
            container_port = "80"
            protocol       = "TCP"
          }

          port {
            container_port = "443"
            protocol       = "TCP"
          }

          resources {
            requests {
              cpu    = "256m"
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

resource "kubernetes_horizontal_pod_autoscaler" "router" {
  metadata {
    namespace = var.namespace
    name      = "router"
  }

  spec {
    min_replicas                      = 1
    max_replicas                      = 1
    target_cpu_utilization_percentage = 100

    scale_target_ref {
      api_version = "apps/v1"
      kind        = "Deployment"
      name        = "router"
    }
  }

  lifecycle {
    ignore_changes = [spec[0].min_replicas, spec[0].max_replicas]
  }
}
