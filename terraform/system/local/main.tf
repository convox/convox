data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  arm_type = module.platform.arch == "arm64"
  current = jsondecode(data.http.releases.response_body).tag_name
  release = local.arm_type ? format("%s-%s", coalesce(var.release, local.current), "arm64") : coalesce(var.release, local.current)
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

  image    = var.image
  name     = var.name
  platform = module.platform.name
  release  = local.release
}
