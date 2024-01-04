module "elasticsearch" {
  source = "../../elasticsearch/k8s-ephemeral"

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

  cluster       = var.cluster_id
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

  buildkit_enabled             = var.buildkit_enabled
  build_node_enabled           = var.build_node_enabled
  docker_hub_authentication    = var.docker_hub_authentication
  domain                       = var.domain
  disable_image_manifest_cache = var.disable_image_manifest_cache
  image                        = var.image
  metrics_scraper_host         = var.metrics_scraper_host
  namespace                    = var.namespace
  rack                         = var.name
  rack_name                    = var.rack_name
  release                      = var.release
  replicas                     = var.high_availability ? 2 : 1
  resolver                     = var.resolver

  annotations = {
    "cert-manager.io/cluster-issuer" = "letsencrypt"
    "cert-manager.io/duration"       = var.cert_duration
    "convox.com/cluster-id" = var.cluster_id // add explicit dependency
  }

  env = {
    ELASTIC_URL     = module.elasticsearch.url
    EXOSCALE_ZONE         = var.zone
    EXOSCALE_ACCESS_KEY   = exoscale_iam_api_key.api_key.key
    EXOSCALE_SECRET_KEY   = exoscale_iam_api_key.api_key.secret
    BUCKET                = aws_s3_bucket.storage_bucket.bucket
    CERT_MANAGER          = "true"
    PROVIDER              = "exoscale"
    REGISTRY        = "registry.${var.domain}"
    REGISTRY_SECRET = var.registry_secret
    RESOLVER              = var.resolver
    ROUTER                = var.router
    SOCKET                = "/var/run/docker.sock"
  }
}
