terraform {
  required_version = ">= 0.12.0"
}

provider "digitalocean" {
  version = "~> 1.9"
}

provider "local" {
  version = "~> 1.3"
}

provider "random" {
  version = "~> 2.2"
}

resource "digitalocean_kubernetes_cluster" "rack" {
  name    = var.name
  region  = var.region
  version = "1.14.8-do.0"

  node_pool {
    name       = "rack"
    size       = var.node_type
    node_count = 3
  }
}

# data "google_client_config" "current" {}

# data "google_container_engine_versions" "available" {
#   location       = data.google_client_config.current.region
#   version_prefix = "1.14."
# }

# data "google_project" "current" {}

# resource "random_string" "password" {
#   length  = 64
#   special = true
# }

# resource "google_container_cluster" "rack" {
#   provider = "google-beta"

#   name     = var.name
#   location = data.google_client_config.current.region

#   remove_default_node_pool = true
#   initial_node_count       = 1
#   logging_service          = "logging.googleapis.com"
#   min_master_version       = data.google_container_engine_versions.available.latest_master_version

#   ip_allocation_policy {
#     use_ip_aliases = true
#   }

#   workload_identity_config {
#     identity_namespace = "${data.google_project.current.project_id}.svc.id.goog"
#   }

#   master_auth {
#     username = "gcloud"
#     password = random_string.password.result

#     client_certificate_config {
#       issue_client_certificate = true
#     }
#   }
# }

# resource "google_container_node_pool" "rack" {
#   provider = "google-beta"

#   name       = "${google_container_cluster.rack.name}-nodes-${var.node_type}"
#   location   = google_container_cluster.rack.location
#   cluster    = google_container_cluster.rack.name
#   node_count = 1

#   node_config {
#     preemptible  = true
#     machine_type = var.node_type

#     metadata = {
#       disable-legacy-endpoints = "true"
#     }

#     workload_metadata_config {
#       node_metadata = "GKE_METADATA_SERVER"
#     }

#     service_account = google_service_account.nodes.email

#     oauth_scopes = [
#       "https://www.googleapis.com/auth/cloud-platform",
#       "https://www.googleapis.com/auth/devstorage.read_write",
#       "https://www.googleapis.com/auth/logging.write",
#       "https://www.googleapis.com/auth/monitoring",
#     ]
#   }

#   lifecycle {
#     create_before_destroy = true
#   }
# }

resource "local_file" "kubeconfig" {
  depends_on = [
    kubernetes_cluster_role_binding.client,
    digitalocean_kubernetes_cluster.rack,
  ]

  filename = pathexpand("~/.kube/config.do.${var.name}")
  content = templatefile("${path.module}/kubeconfig.tpl", {
    ca       = digitalocean_kubernetes_cluster.rack.kube_config[0].cluster_ca_certificate
    endpoint = digitalocean_kubernetes_cluster.rack.endpoint
    token    = digitalocean_kubernetes_cluster.rack.kube_config[0].token
  })

  lifecycle {
    ignore_changes = [content]
  }
}

provider "kubernetes" {
  version = "~> 1.8"

  alias = "direct"

  load_config_file = false

  cluster_ca_certificate = "${base64decode(digitalocean_kubernetes_cluster.rack.kube_config[0].cluster_ca_certificate)}"
  host                   = digitalocean_kubernetes_cluster.rack.endpoint
  token                  = digitalocean_kubernetes_cluster.rack.kube_config[0].token
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
