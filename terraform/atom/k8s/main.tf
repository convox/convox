terraform {
  required_version = ">= 0.12.0"
}

provider "kubernetes" {
  version = "~> 1.8"

  config_path = var.kubeconfig
}

provider "null" {
  version = "~> 2.1"
}

resource "null_resource" "crd" {
  provisioner "local-exec" {
    command = "kubectl apply -f ${path.module}/crd.yml"
    environment = {
      "KUBECONFIG" : var.kubeconfig,
    }
  }
}
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
  }

  spec {
    revision_history_limit = 0

    selector {
      match_labels = {
        system  = "convox"
        service = "atom"
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
          system  = "convox"
          service = "atom"
        }
      }

      spec {
        automount_service_account_token = true
        share_process_namespace         = true
        service_account_name            = "atom"

        container {
          name              = "main"
          args              = ["atom"]
          image             = "convox/convox:${var.release}"
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
