provider "google" {
  project = module.project.id
  region  = var.region
}

provider "kubernetes" {
  client_certificate     = module.cluster.client_certificate
  client_key             = module.cluster.client_key
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint

}

module "project" {
  source = "./project"
}

data "http" "releases" {
  count = var.release == "" ? 1 : 0

  url = "https://api.github.com/repos/${var.image}/releases/latest"
  request_headers = {
    User-Agent = "convox"
  }
}

locals {
  name            = lower(var.name)
  rack_name       = lower(var.rack_name)
  desired_release = var.release != "" ? var.release : jsondecode(data.http.releases[0].response_body).tag_name
  release         = local.desired_release
}

module "cluster" {
  source = "../../cluster/gcp"

  providers = {
    google = google
  }

  k8s_version = var.k8s_version
  name        = local.name
  node_disk   = var.node_disk
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

  buildkit_enabled      = var.buildkit_enabled
  cluster               = module.cluster.id
  docker_hub_username   = var.docker_hub_username
  docker_hub_password   = var.docker_hub_password
  image                 = var.image
  name                  = local.name
  rack_name             = local.rack_name
  network               = module.cluster.network
  nodes_account         = module.cluster.nodes_account
  project_id            = module.project.id
  region                = var.region
  release               = local.release
  syslog                = var.syslog
  telemetry             = var.telemetry
  telemetry_map         = local.telemetry_map
  telemetry_default_map = local.telemetry_default_map
  whitelist             = split(",", var.whitelist)
}
