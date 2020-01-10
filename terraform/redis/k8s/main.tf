provider "kubernetes" {
  version = "~> 1.10"
}

locals {
  host = "${kubernetes_service.redis.metadata.0.name}.${kubernetes_service.redis.metadata.0.namespace}.svc.cluster.local"
  port = "6379"
}

resource "kubernetes_persistent_volume_claim" "data" {
  metadata {
    namespace = var.namespace
    name      = "redis-${var.name}-data"

    labels = {
      system = "convox"
    }
  }

  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        storage = var.disk
      }
    }
  }
}

resource "kubernetes_deployment" "redis" {
  metadata {
    namespace = var.namespace
    name      = "redis-${var.name}"

    labels = {
      system = "convox"
    }
  }

  spec {
    min_ready_seconds      = 1
    revision_history_limit = 0

    selector {
      match_labels = {
        name   = var.name
        scope  = "system"
        system = "convox"
        type   = "redis"
      }
    }

    template {
      metadata {
        labels = {
          name   = var.name
          scope  = "system"
          system = "convox"
          type   = "redis"
        }
      }

      spec {
        container {
          name              = "redis"
          image             = "redis:4.0.10"
          image_pull_policy = "IfNotPresent"

          port {
            container_port = 6379
            protocol       = "TCP"
          }

          volume_mount {
            name       = "data"
            mount_path = "/data"
          }
        }

        volume {
          name = "data"

          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.data.metadata.0.name
          }
        }
      }
    }
  }
}

resource "kubernetes_service" "redis" {
  metadata {
    namespace = var.namespace
    name      = "redis-${var.name}"

    labels = {
      name   = var.name
      scope  = "system"
      system = "convox"
      type   = "redis"
    }
  }

  spec {
    type = "ClusterIP"

    selector = {
      name   = var.name
      scope  = "system"
      system = "convox"
      type   = "redis"
    }

    port {
      port        = 6379
      target_port = 6379
      protocol    = "TCP"
    }
  }
}
