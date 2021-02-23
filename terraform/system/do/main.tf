terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 1.13"
    }
    http = {
      source = "hashicorp/http"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
  }
}

provider "digitalocean" {
  spaces_access_id  = var.access_id
  spaces_secret_key = var.secret_key
}

provider "http" {
  version = "~> 1.1"
}

provider "kubernetes" {
  version = "~> 1.11"

  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint
  token                  = module.cluster.token

  load_config_file = false
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases/latest"
}

locals {
  current = jsondecode(data.http.releases.body).tag_name
  release = coalesce(var.release, local.current)
}

module "cluster" {
  source = "../../cluster/do"

  providers = {
    digitalocean = digitalocean
  }

  name      = var.name
  node_type = var.node_type
  region    = var.region
}

module "rack" {
  source = "../../rack/do"

  providers = {
    digitalocean = digitalocean
    kubernetes   = kubernetes
  }

  access_id     = var.access_id
  cluster       = module.cluster.id
  name          = var.name
  region        = var.region
  registry_disk = var.registry_disk
  release       = local.release
  secret_key    = var.secret_key
  syslog        = var.syslog
  whitelist     = split(",", var.whitelist)
}
