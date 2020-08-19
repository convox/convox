terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 1.13"
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

  domain  = module.router.endpoint
  name    = var.name
  release = var.release
}

module "api" {
  source = "../../api/do"

  providers = {
    digitalocean = digitalocean
    kubernetes   = kubernetes
  }

  access_id  = var.access_id
  cluster    = var.cluster
  domain     = module.router.endpoint
  name       = var.name
  namespace  = module.k8s.namespace
  region     = var.region
  release    = var.release
  resolver   = module.resolver.endpoint
  router     = module.router.endpoint
  secret     = random_string.secret.result
  secret_key = var.secret_key
  syslog     = var.syslog
}

module "resolver" {
  source = "../../resolver/do"

  providers = {
    digitalocean = digitalocean
    kubernetes   = kubernetes
  }

  namespace = module.k8s.namespace
  rack      = var.name
  release   = var.release
}

module "router" {
  source = "../../router/do"

  providers = {
    digitalocean = digitalocean
    kubernetes   = kubernetes
  }

  name      = var.name
  namespace = module.k8s.namespace
  region    = var.region
  release   = var.release
  whitelist = var.whitelist
}
