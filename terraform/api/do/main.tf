provider "digitalocean" {
  version = "~> 1.13"
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

module "elasticsearch" {
  source = "../../elasticsearch/k8s"

  providers = {
    kubernetes = kubernetes
  }

  namespace = var.namespace
}

module "fluentd" {
  source = "../../fluentd/elasticsearch"

  providers = {
    kubernetes = kubernetes
  }

  cluster       = var.cluster
  elasticsearch = module.elasticsearch.host
  namespace     = var.namespace
  rack          = var.name
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain    = var.domain
  namespace = var.namespace
  rack      = var.name
  release   = var.release

  annotations = {}

  env = {
    BUCKET          = digitalocean_spaces_bucket.storage.name
    ELASTIC_URL     = module.elasticsearch.url
    PROVIDER        = "do"
    REGION          = var.region
    REGISTRY        = "registry.${var.domain}"
    RESOLVER        = var.resolver
    ROUTER          = var.router
    SECRET          = var.secret
    SPACES_ACCESS   = var.access_id
    SPACES_ENDPOINT = "https://${var.region}.digitaloceanspaces.com"
    SPACES_SECRET   = var.secret_key
  }
}
