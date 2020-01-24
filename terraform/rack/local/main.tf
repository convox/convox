terraform {
  required_version = ">= 0.12.0"
}

provider "kubernetes" {
  version = "~> 1.10"
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain  = module.router.endpoint
  name    = var.name
  release = var.release
}

module "api" {
  source = "../../api/local"

  providers = {
    kubernetes = kubernetes
  }

  domain    = module.router.endpoint
  name      = var.name
  namespace = module.k8s.namespace
  release   = var.release
  resolver  = module.router.resolver
  router    = module.router.endpoint
  secret    = random_string.secret.result
}

resource "kubernetes_namespace" "system" {
  metadata {
    labels = {
      system = "convox"
      type   = "system"
    }

    name = "convox-system"
  }
}

module "router" {
  source = "../../router/local"

  providers = {
    kubernetes = kubernetes
  }

  name      = var.name
  namespace = kubernetes_namespace.system.metadata.0.name
  platform  = var.platform
  release   = var.release
}
