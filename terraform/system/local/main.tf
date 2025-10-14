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
  arm_type        = module.platform.arch == "arm64"
  desired_release = var.release != "" ? var.release : jsondecode(data.http.releases[0].response_body).tag_name
  release         = local.arm_type ? format("%s-%s", local.desired_release, "arm64") : local.desired_release
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

  docker_hub_username   = var.docker_hub_username
  docker_hub_password   = var.docker_hub_password
  image                 = var.image
  name                  = local.name
  rack_name             = local.rack_name
  platform              = module.platform.name
  os                    = var.os
  release               = local.release
  settings              = var.settings
  telemetry             = var.telemetry
  telemetry_map         = local.telemetry_map
  telemetry_default_map = local.telemetry_default_map
}
