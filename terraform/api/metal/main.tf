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
  syslog        = var.syslog
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  domain    = var.domain
  image     = var.image
  namespace = var.namespace
  rack      = var.name
  release   = var.release
  resolver  = var.resolver

  annotations = {
    "cert-manager.io/cluster-issuer" = "self-signed"
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
