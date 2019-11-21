terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 2.19"
}

provider "google-beta" {
  version = "~> 2.19"
}

provider "local" {
  version = "~> 1.3"
}

provider "random" {
  version = "~> 2.2"
}

data "google_client_config" "current" {}

data "google_container_engine_versions" "available" {
  location       = data.google_client_config.current.region
  version_prefix = "1.14."
}

data "google_project" "current" {}

resource "random_string" "password" {
  length  = 64
  special = true
}

resource "google_container_cluster" "rack" {
  provider = "google-beta"

  name     = var.name
  location = data.google_client_config.current.region
  network  = google_compute_network.rack.name

  remove_default_node_pool = true
  initial_node_count       = 1
  logging_service          = "logging.googleapis.com"
  min_master_version       = data.google_container_engine_versions.available.latest_master_version

  workload_identity_config {
    identity_namespace = "${data.google_project.current.project_id}.svc.id.goog"
  }

  ip_allocation_policy {}

  master_auth {
    username = "gcloud"
    password = random_string.password.result

    client_certificate_config {
      issue_client_certificate = true
    }
  }
}

resource "google_container_node_pool" "rack" {
  provider = "google-beta"

  name       = "${google_container_cluster.rack.name}-nodes-${var.node_type}"
  location   = google_container_cluster.rack.location
  cluster    = google_container_cluster.rack.name
  node_count = 1

  node_config {
    preemptible  = true
    machine_type = var.node_type

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

  lifecycle {
    create_before_destroy = true
  }
}

resource "local_file" "kubeconfig" {
  depends_on = [
    kubernetes_cluster_role_binding.client,
    google_container_node_pool.rack,
  ]

  filename = pathexpand("~/.kube/config.gcp.${var.name}")
  content = templatefile("${path.module}/kubeconfig.tpl", {
    ca                 = google_container_cluster.rack.master_auth.0.cluster_ca_certificate
    endpoint           = google_container_cluster.rack.endpoint
    client_certificate = google_container_cluster.rack.master_auth.0.client_certificate
    client_key         = google_container_cluster.rack.master_auth.0.client_key
  })

  lifecycle {
    ignore_changes = [content]
  }
}

provider "kubernetes" {
  version = "~> 1.8"

  alias = "direct"

  load_config_file = false

  cluster_ca_certificate = "${base64decode(google_container_cluster.rack.master_auth.0.cluster_ca_certificate)}"
  host                   = "https://${google_container_cluster.rack.endpoint}"
  username               = "gcloud"
  password               = random_string.password.result
}

resource "kubernetes_cluster_role_binding" "client" {
  provider = "kubernetes.direct"

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
