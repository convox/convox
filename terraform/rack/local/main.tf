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
  resolver  = module.resolver.endpoint
  router    = module.router.endpoint
  secret    = random_string.secret.result
}

module "metrics" {
  source = "../../metrics/k8s"

  providers = {
    kubernetes = kubernetes
  }

  args = ["--kubelet-insecure-tls"]
}

module "resolver" {
  source = "../../resolver/local"

  providers = {
    kubernetes = kubernetes
  }

  namespace = module.k8s.namespace
  platform  = var.platform
  rack      = var.name
  release   = var.release
}

module "router" {
  source = "../../router/local"

  providers = {
    kubernetes = kubernetes
  }

  name      = var.name
  namespace = module.k8s.namespace
  platform  = var.platform
  release   = var.release
}
