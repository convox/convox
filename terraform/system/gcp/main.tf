terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 2.18"
}

provider "google-beta" {
  version = "~> 2.18"
}

provider "kubernetes" {
  version = "~> 1.9"

  config_path = module.cluster.kubeconfig
}

module "project" {
  source = "./project"

  providers = {
    google = google
  }
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
    google      = google
    google-beta = google-beta
  }

  name      = var.name
  node_type = var.node_type
  services  = module.project.services
}

module "rack" {
  source = "../../rack/gcp"

  providers = {
    kubernetes = kubernetes
    google     = google
  }

  kubeconfig    = module.cluster.kubeconfig
  name          = var.name
  nodes_account = module.cluster.nodes_account
  release       = local.release
}
