provider "digitalocean" {
  spaces_access_id  = var.access_id
  spaces_secret_key = var.secret_key
}

provider "kubernetes" {
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint
  token                  = module.cluster.token

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
  source = "../../cluster/do"

  providers = {
    digitalocean = digitalocean
  }

  high_availability = var.high_availability
  k8s_version       = var.k8s_version
  name              = local.name
  node_type         = var.node_type
  region            = var.region
}

module "rack" {
  source = "../../rack/do"

  providers = {
    digitalocean = digitalocean
    kubernetes   = kubernetes
  }

  access_id             = var.access_id
  buildkit_enabled      = var.buildkit_enabled
  cluster               = module.cluster.id
  docker_hub_username   = var.docker_hub_username
  docker_hub_password   = var.docker_hub_password
  high_availability     = var.high_availability
  image                 = var.image
  name                  = local.name
  rack_name             = local.rack_name
  region                = var.region
  registry_disk         = var.registry_disk
  release               = local.release
  secret_key            = var.secret_key
  syslog                = var.syslog
  telemetry             = var.telemetry
  telemetry_map         = local.telemetry_map
  telemetry_default_map = local.telemetry_default_map
  whitelist             = split(",", var.whitelist)
}
