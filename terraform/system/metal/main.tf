data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  current = jsondecode(data.http.releases.body).tag_name
  release = coalesce(var.release, local.current)
}

provider "kubernetes" {}

provider "helm" {
  kubernetes {
    config_path = "~/.kube/config"
  }
}

module "rack" {
  source = "../../rack/metal"

  providers = {
    helm       = helm
    kubernetes = kubernetes
  }

  domain        = var.domain
  image         = var.image
  name          = var.name
  release       = local.release
  registry_disk = var.registry_disk
  syslog        = var.syslog
  whitelist     = split(",", var.whitelist)
}

