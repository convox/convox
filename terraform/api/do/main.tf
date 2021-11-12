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
  replicas  = var.high_availability ? 2 : 1
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
  syslog        = var.syslog
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_authentication = var.docker_hub_authentication
  domain                    = var.domain
  image                     = var.image
  namespace                 = var.namespace
  rack                      = var.name
  release                   = var.release
  resolver                  = var.resolver
  replicas                  = var.high_availability ? 2 : 1


  annotations = {
    "cert-manager.io/cluster-issuer" = "letsencrypt"
    "kubernetes.io/ingress.class"    = "nginx"
  }

  env = {
    BUCKET          = digitalocean_spaces_bucket.storage.name
    CERT_MANAGER    = "true"
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
