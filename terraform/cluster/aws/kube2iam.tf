resource "kubernetes_cluster_role" "kube2iam" {
  metadata {
    name = "kube2iam"
  }

  rule {
    api_groups = [""]
    resources  = ["namespaces", "pods"]
    verbs      = ["get", "list", "watch"]
  }
}

resource "kubernetes_cluster_role_binding" "kube2iam" {
  metadata {
    name = "kube2iam"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "kube2iam"
  }

  subject {
    kind      = "ServiceAccount"
    name      = "kube2iam"
    namespace = "kube-system"
  }
}

resource "kubernetes_service_account" "kube2iam" {
  metadata {
    namespace = "kube-system"
    name      = "kube2iam"
  }
}

resource "kubernetes_daemonset" "kube2iam" {
  metadata {
    namespace = "kube-system"
    name      = "kube2iam"
  }

  spec {
    selector {
      match_labels = {
        service = "kube2iam"
      }
    }

    template {
      metadata {
        labels = {
          service = "kube2iam"
        }
      }

      spec {
        automount_service_account_token = true
        host_network                    = true
        service_account_name            = "kube2iam"

        container {
          image = "jtblin/kube2iam:latest"
          name  = "kube2iam"
          args = [
            "--base-role-arn=arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/",
            "--host-interface=eni+",
            "--host-ip=$(HOST_IP)",
            "--node=$(NODE_NAME)",
          ]

          env {
            name = "HOST_IP"
            value_from {
              field_ref {
                field_path = "status.podIP"
              }
            }
          }

          env {
            name = "NODE_NAME"
            value_from {
              field_ref {
                field_path = "spec.nodeName"
              }
            }
          }

          port {
            container_port = 8181
            host_port      = 8181
            name           = "http"
          }

          security_context {
            privileged = true
          }
        }
      }
    }
  }
}
