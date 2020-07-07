terraform {
  required_version = ">= 0.12.0"
}

provider "google" {
  version = "~> 3.5.0"

  project = module.project.id
  region  = var.region
}

provider "google-beta" {
  version = "~> 3.5.0"

  project = module.project.id
  region  = var.region
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.10.0"

  client_certificate     = module.cluster.client_certificate
  client_key             = module.cluster.client_key
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint

  load_config_file = false
}

module "project" {
  source = "./project"
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases/latest"
}

locals {
  current = jsondecode(data.http.releases.body).tag_name
  release = coalesce(var.release, local.current)
}

module "cluster" {
  source = "../../cluster/gcp"

  providers = {
    google      = google
    google-beta = google-beta
  }

  name        = var.name
  node_type   = var.node_type
  preemptible = var.preemptible
}

module "rack" {
  source = "../../rack/gcp"

  providers = {
    kubernetes = kubernetes
    google     = google
  }

  cluster       = module.cluster.id
  name          = var.name
  network       = module.cluster.network
  nodes_account = module.cluster.nodes_account
  release       = local.release
  whitelist     = split(",", var.whitelist)
}
