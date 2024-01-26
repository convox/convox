data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  name      = lower(var.name)
  rack_name = lower(var.rack_name)
  current   = jsondecode(data.http.releases.response_body).tag_name
  release   = coalesce(var.release, local.current)
}

provider "kubernetes" {}

module "rack" {
  source = "../../rack/metal"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_username = var.docker_hub_username
  docker_hub_password = var.docker_hub_password
  domain              = var.domain
  image               = var.image
  name                = local.name
  release             = local.release
  registry_disk       = var.registry_disk
  syslog              = var.syslog
  whitelist           = split(",", var.whitelist)
}

