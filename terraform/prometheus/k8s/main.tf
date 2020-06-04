provider "kubernetes" {
  version = "~> 1.10"
}

resource "kubernetes_cluster_role" "prometheus" {
  metadata {
    name = "prometheus"
  }

  rule {
    api_groups = [""]
    resources  = ["endpoints", "nodes", "nodes/proxy", "pods", "services"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = ["extensions"]
    resources  = ["ingresses"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    non_resource_urls = ["/metrics"]
    verbs             = ["get"]
  }
}

resource "kubernetes_service_account" "prometheus" {
  metadata {
    name      = "prometheus"
    namespace = var.namespace
  }
}

resource "kubernetes_cluster_role_binding" "prometheus" {
  metadata {
    name = "prometheus"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = kubernetes_cluster_role.prometheus.metadata[0].name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account.prometheus.metadata[0].name
    namespace = kubernetes_service_account.prometheus.metadata[0].namespace
  }
}

resource "kubernetes_config_map" "config" {
  metadata {
    namespace = var.namespace
    name      = "prometheus-config"
  }

  data = {
    "prometheus.yml" = templatefile("${path.module}/prometheus.yml", { namespace = var.namespace }),
    "rules.yml"      = file("${path.module}/rules.yml"),
  }
}

resource "kubernetes_persistent_volume_claim" "storage" {
  metadata {
    namespace = var.namespace
    name      = "prometheus-storage"
  }

  spec {
    access_modes = ["ReadWriteOnce"]

    resources {
      requests = {
        storage = "2Gi"
      }
    }
  }
}

resource "kubernetes_deployment" "prometheus" {
  metadata {
    namespace = var.namespace
    name      = "prometheus"
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        app = "prometheus"
      }
    }

    strategy {
      rolling_update {
        max_surge       = "0%"
        max_unavailable = "100%"
      }
    }

    template {
      metadata {
        labels = {
          app = "prometheus"
        }
      }

      spec {
        automount_service_account_token = true
        service_account_name            = "prometheus"

        container {
          name  = "prometheus"
          image = "prom/prometheus:v2.18.1"

          port {
            container_port = 9090
            name           = "default"
          }

          volume_mount {
            name       = "config"
            mount_path = "/etc/prometheus"
          }

          volume_mount {
            name       = "storage"
            mount_path = "/prometheus"
          }
        }

        security_context {
          fs_group        = "2000"
          run_as_user     = "1000"
          run_as_non_root = true
        }

        volume {
          name = "config"

          config_map {
            name = kubernetes_config_map.config.metadata[0].name
          }
        }

        volume {
          name = "storage"

          persistent_volume_claim {
            claim_name = "prometheus-storage"
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "prometheus" {
  metadata {
    namespace = var.namespace
    name      = "prometheus"
  }

  spec {
    type = "ClusterIP"

    selector = {
      app = "prometheus"
    }

    port {
      protocol    = "TCP"
      port        = 9090
      target_port = 9090
    }
  }
}
