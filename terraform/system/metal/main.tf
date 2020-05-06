terraform {
  required_version = ">= 0.12.0"
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.10.0"
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases/latest"
}

locals {
  current = jsondecode(data.http.releases.body).tag_name
  release = coalesce(var.release, local.current)
}

module "rack" {
  source = "../../rack/metal"

  providers = {
    kubernetes = kubernetes
  }

  domain        = var.domain
  name          = var.name
  release       = local.release
  registry_disk = var.registry_disk
}

