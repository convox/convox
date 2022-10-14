resource "random_string" "secret" {
  length  = 30
  special = false
}

resource "kubernetes_deployment" "registry" {
  depends_on = [kubernetes_persistent_volume_claim.registry]

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
    name      = "registry-ing-v1"

    annotations = {
      "cert-manager.io/cluster-issuer" = "self-signed"
    }

    labels = {
      system  = "convox"
      service = "registry"
    }
  }

  spec {
    ingress_class_name = "nginx"
    tls {
      hosts       = ["registry.${local.endpoint}"]
      secret_name = "registry-certificate"
    }

    rule {
      host = "registry.${local.endpoint}"

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

