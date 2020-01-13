terraform {
  required_version = ">= 0.12.0"
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.10"
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases"
}

locals {
  current = jsondecode(data.http.releases.body).0.tag_name
  release = coalesce(var.release, local.current)
}

module "platform" {
  source = "../../platform"
}

module "rack" {
  source = "../../rack/local"

  providers = {
    kubernetes = kubernetes
  }

  name     = var.name
  platform = module.platform.name
  release  = local.release
}
