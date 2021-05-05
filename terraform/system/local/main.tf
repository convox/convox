data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  current = jsondecode(data.http.releases.body).tag_name
  release = coalesce(var.release, local.current)
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
