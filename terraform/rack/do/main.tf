terraform {
  required_version = ">= 0.12.0"
}

provider "digitalocean" {
  version = "~> 1.9"
}

provider "kubernetes" {
  version = "~> 1.9"
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

  access_id     = var.access_id
  elasticsearch = module.elasticsearch.url
  domain        = module.router.endpoint
  name          = var.name
  namespace     = module.k8s.namespace
  region        = var.region
  release       = var.release
  router        = module.router.endpoint
  secret        = random_string.secret.result
  secret_key    = var.secret_key
}

module "elasticsearch" {
  source = "../../elasticsearch/k8s"

  providers = {
    kubernetes = kubernetes
  }

  namespace = module.k8s.namespace
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
}
