resource "kubernetes_cluster_role" "atom" {
  metadata {
    name = "atom"
  }

  rule {
    api_groups = ["*"]
    resources  = ["*"]
    verbs      = ["*"]
  }
}

resource "kubernetes_cluster_role_binding" "atom" {
  metadata {
    name = "atom"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "atom"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "atom"
    namespace = var.namespace
  }
}

resource "kubernetes_service_account" "atom" {
  metadata {
    namespace = var.namespace
    name      = "atom"
  }
}

resource "kubernetes_deployment" "atom" {
  metadata {
    namespace = var.namespace
    name      = "atom"

    labels = {
      app     = "system"
      name    = "atom"
      rack    = var.rack
      service = "atom"
      system  = "convox"
      type    = "service"
    }
  }

  spec {
    revision_history_limit = 0

    selector {
      match_labels = {
        name    = "atom"
        service = "atom"
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
        annotations = {
          "scheduler.alpha.kubernetes.io/critical-pod" : ""
        }

        labels = {
          app     = "system"
          name    = "atom"
          rack    = var.rack
          service = "atom"
          system  = "convox"
          type    = "service"
        }
      }

      spec {
        automount_service_account_token = true
        share_process_namespace         = true
        service_account_name            = "atom"
        priority_class_name             = var.set_priority_class ? "system-cluster-critical" : null

        container {
          name              = "system"
          args              = ["atom"]
          image             = "${var.image}:${var.release}"
          image_pull_policy = "Always"

          resources {
            requests {
              cpu    = "32m"
              memory = "32Mi"
            }
          }
        }
      }
    }
  }
}
