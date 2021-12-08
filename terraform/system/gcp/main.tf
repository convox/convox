provider "google" {
  project = module.project.id
  region  = var.region
}

provider "kubernetes" {
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
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  current = jsondecode(data.http.releases.body).tag_name
  release = coalesce(var.release, local.current)
}

module "cluster" {
  source = "../../cluster/gcp"

  providers = {
    google = google
  }

  name        = var.name
  node_type   = var.node_type
  preemptible = var.preemptible
  project_id  = module.project.id
}

module "rack" {
  source = "../../rack/gcp"

  providers = {
    kubernetes = kubernetes
    google     = google
  }

  cluster             = module.cluster.id
  docker_hub_username = var.docker_hub_username
  docker_hub_password = var.docker_hub_password
  image               = var.image
  name                = var.name
  network             = module.cluster.network
  nodes_account       = module.cluster.nodes_account
  project_id          = module.project.id
  region              = var.region
  release             = local.release
  syslog              = var.syslog
  whitelist           = split(",", var.whitelist)
}
