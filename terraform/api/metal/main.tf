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

  cluster                  = var.name
  elasticsearch            = module.elasticsearch.host
  fluentd_memory           = var.fluentd_memory
  namespace                = var.namespace
  rack                     = var.name
  router_log_redact_params = var.router_log_redact_params
  syslog                   = var.syslog
  syslog_tls_verify        = var.syslog_tls_verify
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
  private_api               = var.private_api
  resolver                  = var.resolver
  webhook_signing_key       = var.webhook_signing_key

  annotations = {
    "cert-manager.io/cluster-issuer" = "self-signed"
  }

  env = {
    CERT_MANAGER       = "true"
    ELASTIC_URL        = module.elasticsearch.url
    ELASTICSEARCH_DISK = var.elasticsearch_disk
    PROVIDER           = "metal"
    REGISTRY           = "registry.${var.domain}"
    RESOLVER           = var.resolver
    ROUTER             = var.router
    SECRET             = var.secret
  }

  volumes = {
    storage = "/var/storage"
  }
}
