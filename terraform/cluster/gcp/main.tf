provider "google" {
  version = "~> 3.5.0"
}

provider "google-beta" {
  version = "~> 3.5.0"
}

provider "local" {
  version = "~> 1.3"
}

provider "random" {
  version = "~> 2.2"
}

data "google_client_config" "current" {}

data "google_project" "current" {}

resource "random_string" "password" {
  length  = 64
  special = true
}

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

  master_auth {
    username = "gcloud"
    password = random_string.password.result

    client_certificate_config {
      issue_client_certificate = true
    }
  }
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

    upgrade_settings {
      max_surge       = 1
      max_unavailable = 1
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
  version = "~> 1.10"

  alias = "direct"

  load_config_file = false

  cluster_ca_certificate = base64decode(google_container_cluster.rack.master_auth.0.cluster_ca_certificate)
  host                   = "https://${google_container_cluster.rack.endpoint}"
  username               = "gcloud"
  password               = random_string.password.result
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
