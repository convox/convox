terraform {
  required_version = ">= 0.12.0"
}

provider "kubernetes" {
  version = "~> 1.10"
}

locals {
  endpoint = coalesce(var.domain, module.router.endpoint)
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
  source = "../../api/metal"

  providers = {
    kubernetes = kubernetes
  }

  cert_manager = var.domain != ""
  domain       = local.endpoint
  name         = var.name
  namespace    = module.k8s.namespace
  release      = var.release
  resolver     = module.resolver.endpoint
  router       = module.router.endpoint
  secret       = random_string.secret.result
}

module "resolver" {
  source = "../../resolver/metal"

  providers = {
    kubernetes = kubernetes
  }

  namespace = module.k8s.namespace
  rack      = var.name
  release   = var.release
}

module "router" {
  source = "../../router/metal"

  providers = {
    kubernetes = kubernetes
  }

  name      = var.name
  namespace = module.k8s.namespace
  release   = var.release
}
