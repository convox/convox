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

  authentication = false
  domain         = var.domain
  namespace      = var.namespace
  rack           = var.name
  release        = var.release
  replicas       = 1

  annotations = {
    "convox.com/idles"            = "true"
    "kubernetes.io/ingress.class" = "convox"
  }

  env = {
    PROVIDER = "local"
    REGISTRY = "registry.${var.domain}"
    RESOLVER = var.resolver
    ROUTER   = var.router
    SECRET   = var.secret
    STORAGE  = "/var/storage"
  }

  volumes = {
    storage = "/var/storage"
  }
}
