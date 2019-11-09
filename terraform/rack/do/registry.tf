resource "random_string" "suffix" {
  length  = 12
  special = false
  upper   = false
}

resource "digitalocean_spaces_bucket" "registry" {
  name          = "${var.name}-registry-${random_string.suffix.result}"
  region        = var.region
  acl           = "private"
  force_destroy = true
}

resource "random_string" "secret" {
  length = 30
}

resource "kubernetes_deployment" "registry" {
  metadata {
    namespace = module.k8s.namespace
    name      = "registry"

    labels = {
      serivce = "registry"
    }
  }

  spec {
    min_ready_seconds      = 1
    revision_history_limit = 0

    selector {
      match_labels = {
        system  = "convox"
        service = "registry"
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
        labels = {
          system  = "convox"
          service = "registry"
        }
      }

      spec {
        container {
          name              = "main"
          image             = "registry:2"
          image_pull_policy = "IfNotPresent"

          env {
            name  = "REGISTRY_HTTP_SECRET"
            value = random_string.secret.result
          }

          env {
            name  = "REGISTRY_STORAGE"
            value = "s3"
          }

          env {
            name  = "REGISTRY_STORAGE_S3_ACCESSKEY"
            value = var.access_id
          }

          env {
            name  = "REGISTRY_STORAGE_S3_BUCKET"
            value = digitalocean_spaces_bucket.registry.name
          }

          env {
            name  = "REGISTRY_STORAGE_S3_REGION"
            value = var.region
          }

          env {
            name  = "REGISTRY_STORAGE_S3_REGIONENDPOINT"
            value = "https://${var.region}.digitaloceanspaces.com"
          }

          env {
            name  = "REGISTRY_STORAGE_S3_SECRETKEY"
            value = var.secret_key
          }

          port {
            container_port = 5000
            protocol       = "TCP"
          }

          volume_mount {
            name       = "registry"
            mount_path = "/var/lib/registry"
          }
        }

        volume {
          name = "registry"

          persistent_volume_claim {
            claim_name = kubernetes_persistent_volume_claim.registry.metadata.0.name
          }
        }
      }
    }
  }
}

resource "kubernetes_persistent_volume_claim" "registry" {
  metadata {
    namespace = module.k8s.namespace
    name      = "registry"
  }

  spec {
    access_modes = ["ReadWriteOnce"]
    resources {
      requests = {
        storage = var.registry_disk
      }
    }
  }
}

resource "kubernetes_service" "registry" {
  metadata {
    namespace = module.k8s.namespace
    name      = "registry"
  }

  spec {
    type = "ClusterIP"

    selector = {
      system  = "convox"
      service = "registry"
    }

    port {
      name        = "http"
      port        = 80
      target_port = 5000
      protocol    = "TCP"
    }
  }
}
resource "kubernetes_ingress" "registry" {
  metadata {
    namespace = module.k8s.namespace
    name      = "registry"

    annotations = {
      "convox.idles" : "true"
    }

    labels = {
      system  = "convox"
      service = "registry"
    }
  }

  spec {
    tls {
      hosts = ["registry.${module.router.endpoint}"]
    }

    rule {
      host = "registry.${module.router.endpoint}"

      http {
        path {
          backend {
            service_name = kubernetes_service.registry.metadata.0.name
            service_port = 80
          }
        }
      }
    }
  }
}

