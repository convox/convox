resource "kubernetes_persistent_volume_claim" "storage" {
  metadata {
    namespace = var.namespace
    name      = "api-storage"

    labels = {
      app     = "system"
      rack    = var.name
      service = "api"
      system  = "convox"
    }
  }

  spec {
    access_modes       = ["ReadWriteMany"]
    storage_class_name = "rook-ceph-shared"

    resources {
      requests = {
        storage = "100Gi"
      }
    }
  }
}
