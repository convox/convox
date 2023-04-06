data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  arm_type = module.platform.arch == "arm64"
  current  = jsondecode(data.http.releases.response_body).tag_name
  release  = local.arm_type ? format("%s-%s", coalesce(var.release, local.current), "arm64") : coalesce(var.release, local.current)

  params            = file("${var.settings}/vars.json")
  send_telemetry    = jsondecode(local.params).telemetry == "false" ? false : true
  telemetry_config  = {
    url              = "https://telemetry.convox.com/telemetry"
    method           = "POST"
    request_headers  = { Accept = "application/json" }
    request_body     = local.params
  }
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
  platform            = module.platform.name
  os                  = var.os
  release             = local.release
}

resource "null_resource" "telemetry" {
  count = local.send_telemetry ? 1 : 0

  provisioner "local-exec" {
    command = <<EOF
      curl -X ${local.telemetry_config.method} \
           -H 'Content-Type: application/json' \
           -H 'Accept: application/json' \
           -d '${local.telemetry_config.request_body}' \
           ${local.telemetry_config.url}
    EOF
  }
}