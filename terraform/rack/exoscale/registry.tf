resource "random_string" "suffix" {
  length  = 10
  special = false
  upper   = false
}

locals {
  bucket_name = "${var.name}-registry-${random_string.suffix.result}"
  s3_region_endpoint = "https://sos-${var.zone}.exo.io"
}

resource "exoscale_iam_role" "sos_admin_role" {
  name = "${var.name}-sos-admin-role"
  description = "SOS registry bucket admin role"
  editable = true

  policy = {
    default_service_strategy = "deny"
    services = {
      sos = {
        type = "rules"
        rules = [
          {
            expression = "parameters.bucket == '${local.bucket_name}'"
            action = "allow"
          }
        ]
      }
    }
  }
}

resource "exoscale_iam_api_key" "sos_api_key" {
  name = "${var.name}-sos-api-key"
  role_id = exoscale_iam_role.sos_admin_role.id
}

resource "aws_s3_bucket" "registry_bucket" {
  bucket   = local.bucket_name
}

resource "aws_s3_bucket_acl" "registry_bucket_acl" {
  bucket = aws_s3_bucket.registry_bucket.id
  acl    = "private"
}

resource "random_string" "secret" {
  length  = 30
  special = false
}

resource "kubernetes_deployment" "registry" {
  depends_on = [
    aws_s3_bucket.registry_bucket
  ]

  metadata {
    namespace = module.k8s.namespace
    name      = "registry"

    labels = {
      app     = "system"
      name    = "registry"
      rack    = var.name
      service = "registry"
      system  = "convox"
      type    = "service"
    }
  }

  spec {
    min_ready_seconds      = 1
    revision_history_limit = 0

    selector {
      match_labels = {
        service = "registry"
        system  = "convox"
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
          app     = "system"
          name    = "registry"
          rack    = var.name
          service = "registry"
          system  = "convox"
          type    = "service"
        }
      }

      spec {
        priority_class_name = "system-cluster-critical"

        container {
          name              = "system"
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
            value = exoscale_iam_api_key.sos_api_key.key
          }

          env {
            name  = "REGISTRY_STORAGE_S3_BUCKET"
            value = aws_s3_bucket.registry_bucket.bucket
          }

          env {
            name  = "REGISTRY_STORAGE_S3_REGION"
            value = var.zone
          }

          env {
            name  = "REGISTRY_STORAGE_S3_REGIONENDPOINT"
            value = local.s3_region_endpoint
          }

          env {
            name  = "REGISTRY_STORAGE_S3_SECRETKEY"
            value = exoscale_iam_api_key.sos_api_key.secret
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
resource "kubernetes_ingress_v1" "registry" {
  wait_for_load_balancer = true

  metadata {
    namespace = module.k8s.namespace
    name      = "registry"

    annotations = {
      "cert-manager.io/cluster-issuer" = "letsencrypt"
    }

    labels = {
      system  = "convox"
      service = "registry"
    }
  }

  spec {
    ingress_class_name = "nginx"
    tls {
      hosts       = ["registry.${module.router.endpoint}"]
      secret_name = "registry-certificate"
    }

    rule {
      host = "registry.${module.router.endpoint}"

      http {
        path {
          backend {
            service {
              name = kubernetes_service.registry.metadata.0.name
              port {
                number = 80
              }
            }
          }
        }
      }
    }
  }
}
