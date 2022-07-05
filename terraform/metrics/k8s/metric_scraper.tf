resource "kubernetes_cluster_role" "metrics_scraper" {
  metadata {
    name = "metrics-scraper"
  }

  rule {
    api_groups = ["metrics.k8s.io"]
    resources  = ["pods", "nodes"]
    verbs      = ["get", "list", "watch"]
  }
}

resource "kubernetes_cluster_role_binding" "metrics_scraper" {
  metadata {
    name = "metrics-scraper"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = kubernetes_cluster_role.metrics_scraper.metadata.0.name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account.metrics_scraper.metadata.0.name
    namespace = kubernetes_service_account.metrics_scraper.metadata.0.namespace
  }
}

resource "kubernetes_service_account" "metrics_scraper" {
  metadata {
    namespace = "kube-system"
    name      = "metrics-scraper"
  }
}

resource "kubernetes_deployment" "metrics_scraper" {
  metadata {
    name      = "metrics-scraper"
    namespace = "kube-system"

    labels = {
      "k8s-app" : "metrics-scraper"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        "k8s-app" : "metrics-scraper"
      }
    }

    template {
      metadata {
        name = "metrics-scraper"
        labels = {
          "k8s-app" = "metrics-scraper"
        }
      }

      spec {
        automount_service_account_token = true
        service_account_name            = kubernetes_service_account.metrics_scraper.metadata.0.name

        container {
          name              = "metrics-scraper"
          image             = "kubernetesui/metrics-scraper:v1.0.7"
          image_pull_policy = "IfNotPresent"

          args = ["--metric-resolution=${var.scraper_metric_resolution}", "--metric-duration=${var.scraper_metric_duration}"]

          port {
            container_port = 8000
          }

          liveness_probe {
            http_get {
              path   = "/"
              port   = 8000
              scheme = "HTTP"
            }

            failure_threshold     = 5
            initial_delay_seconds = 30
            period_seconds        = 3
            success_threshold     = 1
            timeout_seconds       = 5
          }

          volume_mount {
            name       = "tmp-dir"
            mount_path = "/tmp"
          }

          security_context {
            allow_privilege_escalation = false
            read_only_root_filesystem  = true
            run_as_user                = 1001
            run_as_group               = 2001
          }
        }

        volume {
          name = "tmp-dir"
          empty_dir {}
        }
      }
    }
  }
}

resource "kubernetes_service" "metrics_scraper" {
  metadata {
    name      = "metrics-scraper"
    namespace = "kube-system"

    labels = {
      "k8s-app" = "metrics-scraper"
    }
  }

  spec {
    selector = {
      "k8s-app" = "metrics-scraper"
    }

    port {
      port        = 8000
      protocol    = "TCP"
      target_port = 8000
    }
  }
}
