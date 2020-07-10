provider "kubernetes" {
  version = "~> 1.11"
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

  cluster       = var.name
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

  annotations = {
    "cert-manager.io/cluster-issuer" = "letsencrypt"
    "kubernetes.io/ingress.class"    = "nginx"
  }

  env = {
    CERT_MANAGER = var.cert_manager # ? "true" : "false"
    ELASTIC_URL  = module.elasticsearch.url
    PROVIDER     = "metal"
    REGISTRY     = "registry.${var.domain}"
    RESOLVER     = var.resolver
    ROUTER       = var.router
    SECRET       = var.secret
  }

  volumes = {
    storage = "/var/storage"
  }
}
