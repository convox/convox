terraform {
  required_version = ">= 0.12.0"
}

provider "kubernetes" {
  version = "~> 1.10"
}

locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain    = var.domain
  name      = var.name
  namespace = var.namespace
  release   = var.release
  replicas  = 1

  annotations = {}

  env = {
    PROVIDER = "local"
    REGISTRY = "registry.${var.domain}"
    RESOLVER = var.resolver
    ROUTER   = var.router
    SECRET   = var.secret
  }
}
