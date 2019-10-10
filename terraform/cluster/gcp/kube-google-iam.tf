resource "kubernetes_cluster_role" "kube-google-iam" {
  metadata {
    name = "kube-google-iam"
  }

  rule {
    api_groups = [""]
    resources  = ["namespaces", "pods"]
    verbs      = ["get", "list", "watch"]
  }
}

resource "kubernetes_cluster_role_binding" "kube-google-iam" {
  metadata {
    name = "kube-google-iam"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "kube-google-iam"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "kube-google-iam"
    namespace = "kube-system"
  }
}

resource "kubernetes_service_account" "kube-google-iam" {
  metadata {
    namespace = "kube-system"
    name      = "kube-google-iam"
  }
}

resource "kubernetes_daemonset" "kube-google-iam" {
  metadata {
    namespace = "kube-system"
    name      = "kube-google-iam"
  }

  spec {
    selector {
      match_labels = {
        service = "kube-google-iam"
      }
    }

    template {
      metadata {
        labels = {
          service = "kube-google-iam"
        }
      }

      spec {
        automount_service_account_token = true
        host_network                    = true
        service_account_name            = "kube-google-iam"

        toleration {
          key    = "node-role.kubernetes.io/master"
          effect = "NoSchedule"
        }

        container {
          image = "convox/kube-google-iam"
          name  = "kube-google-iam"

          args = [
            "--verbose",
            "--iptables=true",
            "--host-interface=cbr0",
            "--host-ip=$(HOST_IP)",
            "--attributes=cluster-name",
            "--default-service-account=${google_service_account.nodes.email}"
          ]

          image_pull_policy = "Always"

          env {
            name = "HOST_IP"
            value_from {
              field_ref {
                field_path = "status.podIP"
              }
            }
          }

          # port {
          #   container_port = 8181
          #   host_port      = 8181
          #   name           = "http"
          # }

          security_context {
            privileged = true
          }
        }
      }
    }
  }
}

