terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 1.13"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
  }
}

provider "digitalocean" {}

provider "kubernetes" {
  version = "~> 1.11"
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  namespace = var.namespace
  rack      = var.rack
  release   = var.release
}
