data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  arm_type = module.platform.arch == "arm64"
  current  = jsondecode(data.http.releases.response_body).tag_name
  release  = local.arm_type ? format("%s-%s", coalesce(var.release, local.current), "arm64") : coalesce(var.release, local.current)
}

provider "kubernetes" {
  config_path = "~/.kube/config"
}

module "platform" {
  source = "../../platform"
}

module "rack" {
  source = "../../rack/local"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_username = var.docker_hub_username
  docker_hub_password = var.docker_hub_password
  image               = var.image
  name                = var.name
  rack_name           = var.rack_name
  platform            = module.platform.name
  os                  = var.os
  release             = local.release
  settings            = var.settings
  telemetry           = var.telemetry
}
