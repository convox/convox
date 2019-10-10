terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 2.12"
}

provider "kubernetes" {
  version = "~> 1.8"

  config_path = module.cluster.kubeconfig
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases"
}

locals {
  current = jsondecode(data.http.releases.body).0.tag_name
  release = coalesce(var.release, local.current)
}

module "cluster" {
  source = "../../cluster/gcp"

  providers = {
    google = google
  }

  name      = var.name
  node_type = var.node_type
}

module "rack" {
  source = "../../rack/gcp"

  providers = {
    google     = google
    kubernetes = kubernetes
  }

  domain        = var.domain
  kubeconfig    = module.cluster.kubeconfig
  name          = var.name
  nodes_account = module.cluster.nodes_account
  release       = local.release
}
