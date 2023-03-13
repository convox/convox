resource "kubernetes_cluster_role" "fluentd" {
  metadata {
    name = "${var.rack}-fluentd"
  }

  rule {
    api_groups = [""]
    resources  = ["namespaces", "pods", "pods/logs"]
    verbs      = ["get", "list", "watch"]
  }
}

resource "kubernetes_cluster_role_binding" "fluentd" {
  metadata {
    name = "${var.rack}-fluentd"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = kubernetes_cluster_role.fluentd.metadata[0].name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account.fluentd.metadata[0].name
    namespace = kubernetes_service_account.fluentd.metadata[0].namespace
  }
}

resource "kubernetes_service_account" "fluentd" {
  metadata {
    namespace = var.namespace
    name      = "fluentd"

    annotations = var.annotations
  }
}

resource "kubernetes_config_map" "fluentd" {
  metadata {
    namespace = var.namespace
    name      = "fluentd"
  }

  data = {
    "fluent.conf"     = file("${path.module}/fluent.conf")
    "containers.conf" = file("${path.module}/containers.conf")
    "target.conf"     = var.target
  }
}

resource "kubernetes_daemonset" "fluentd" {
  metadata {
    namespace = var.namespace
    name      = "fluentd"

    labels = {
      app     = "system"
      name    = "fluentd"
      rack    = var.rack
      service = "fluentd"
      system  = "convox"
      type    = "service"
    }
  }

  spec {
    selector {
      match_labels = {
        service = "fluentd"
      }
    }

    template {
      metadata {
        labels = {
          app     = "system"
          name    = "fluentd"
          rack    = var.rack
          service = "fluentd"
          system  = "convox"
          type    = "service"
        }

        annotations = var.annotations
      }

      spec {
        service_account_name            = "fluentd"
        automount_service_account_token = true

        init_container {
          name    = "config"
          image   = "busybox"
          command = ["sh", "-c", "cp /config/..data/* /fluentd/etc"]

          volume_mount {
            name       = "config"
            mount_path = "/config"
          }

          volume_mount {
            name       = "fluentd-etc"
            mount_path = "/fluentd/etc"
          }
        }

        container {
          name              = "system"
          image             = var.image
          image_pull_policy = "IfNotPresent"

          env {
            name  = "CLUSTER_NAME"
            value = var.cluster
          }

          env {
            name  = "TARGET_HASH"
            value = sha256(var.target)
          }

          dynamic "env" {
            for_each = var.env

            content {
              name  = env.key
              value = env.value
            }
          }

          resources {
            limits = {
              memory = "200Mi"
            }

            requests = {
              cpu    = "100m"
              memory = "200Mi"
            }
          }

          volume_mount {
            name       = "fluentd-etc"
            mount_path = "/fluentd/etc"
          }

          volume_mount {
            name       = "var-log"
            mount_path = "/var/log"
          }

          volume_mount {
            name       = "var-lib-docker-containers"
            mount_path = "/var/lib/docker/containers"
            read_only  = true
          }

          volume_mount {
            name       = "run-log-journal"
            mount_path = "/run/log/journal"
            read_only  = true
          }

          volume_mount {
            name       = "var-log-dmesg"
            mount_path = "/var/log/dmesg"
            read_only  = true
          }
        }

        volume {
          name = "config"
          config_map {
            name = "fluentd"
          }
        }

        volume {
          name = "fluentd-etc"
          empty_dir {}
        }

        volume {
          name = "var-log"
          host_path {
            path = "/var/log"
          }
        }

        volume {
          name = "var-lib-docker-containers"
          host_path {
            path = "/var/lib/docker/containers"
          }
        }

        volume {
          name = "run-log-journal"
          host_path {
            path = "/run/log/journal"
          }
        }

        volume {
          name = "var-log-dmesg"
          host_path {
            path = "/var/log/dmesg"
          }
        }
      }
    }
  }
}
