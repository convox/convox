data "google_client_config" "current" {}

data "google_project" "current" {}

resource "random_string" "password" {
  length  = 64
  special = true
}

data "google_client_config" "provider" {}

resource "google_container_cluster" "rack" {
  provider = google-beta

  name     = var.name
  location = data.google_client_config.current.region
  network  = google_compute_network.rack.name

  remove_default_node_pool = true
  initial_node_count       = 1

  release_channel {
    channel = "REGULAR"
  }

  workload_identity_config {
    identity_namespace = "${data.google_project.current.project_id}.svc.id.goog"
  }

  ip_allocation_policy {}

}

resource "google_container_node_pool" "rack" {
  provider = google-beta

  name               = "${google_container_cluster.rack.name}-nodes-${var.node_type}"
  location           = google_container_cluster.rack.location
  cluster            = google_container_cluster.rack.name
  initial_node_count = 1

  autoscaling {
    min_node_count = 1
    max_node_count = 1000
  }

  node_config {
    machine_type = var.node_type
    preemptible  = var.preemptible

    metadata = {
      disable-legacy-endpoints = "true"
    }

    workload_metadata_config {
      node_metadata = "GKE_METADATA_SERVER"
    }

    service_account = google_service_account.nodes.email

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
      "https://www.googleapis.com/auth/devstorage.read_write",
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
    ]
  }

  upgrade_settings {
    max_surge       = 1
    max_unavailable = 1
  }

  lifecycle {
    create_before_destroy = true
  }
}

# resource "local_file" "kubeconfig" {
#   depends_on = [
#     kubernetes_cluster_role_binding.client,
#     google_container_node_pool.rack,
#   ]

#   filename = pathexpand("~/.kube/config.gcp.${var.name}")
#   content  = var.kubeconfig_raw

#   lifecycle {
#     ignore_changes = [content]
#   }
# }

provider "kubernetes" {
  host  = "https://${data.google_container_cluster.rack.endpoint}"
  token = data.google_client_config.provider.access_token
  cluster_ca_certificate = base64decode(
    data.google_container_cluster.rack.master_auth[0].cluster_ca_certificate,
  )
}

resource "kubernetes_cluster_role_binding" "client" {
  provider = kubernetes.direct

  metadata {
    name = "client-binding"
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "cluster-admin"
  }

  subject {
    kind = "User"
    name = "client"
  }
}
