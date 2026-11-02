resource "kubernetes_storage_class_v1" "filestore" {
  count    = var.filestore_enable ? 1 : 0
  provider = kubernetes.direct

  depends_on = [google_container_cluster.rack]

  metadata {
    name = "filestore-rwx"
  }

  storage_provisioner    = "filestore.csi.storage.gke.io"
  reclaim_policy         = "Delete"
  volume_binding_mode    = "WaitForFirstConsumer"
  allow_volume_expansion = true

  parameters = {
    tier    = "standard"
    network = google_compute_network.rack.name
  }
}
