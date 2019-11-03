terraform {
  required_version = ">= 0.12.0"
}

provider "digitalocean" {
  version = "~> 1.9"
}

provider "kubernetes" {
  version = "~> 1.8"

  config_path = var.kubeconfig
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

  domain     = var.domain
  kubeconfig = var.kubeconfig
  name       = var.name
  namespace  = var.namespace
  release    = var.release

  annotations = {}

  env = {
    BUCKET            = digitalocean_spaces_bucket.storage.name
    ELASTICSEARCH_URL = "http://elasticsearch.kube-system.svc.cluster.local:9200"
    PROVIDER          = "do"
    REGION            = var.region
    REGISTRY          = "registry.${var.domain}"
    ROUTER            = var.router
    SECRET            = var.secret
    SPACES_ACCESS     = var.access_id
    SPACES_ENDPOINT   = "https://${var.region}.digitaloceanspaces.com"
    SPACES_SECRET     = var.secret_key
  }
}
