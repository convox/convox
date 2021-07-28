resource "kubernetes_persistent_volume_claim" "storage" {
  metadata {
    namespace = var.namespace
    name      = "api-storage-local"

    labels = {
      app     = "system"
      rack    = var.name
      service = "api"
      system  = "convox"
    }
  }

  spec {
    access_modes = ["ReadWriteOnce"]

    resources {
      requests {
        storage = "5Gi"
      }
    }
  }
}
